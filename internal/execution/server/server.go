package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/execution/adapters/db/repository"
	"github.com/linkflow-go/internal/execution/adapters/http/handlers"
	"github.com/linkflow-go/internal/execution/app/orchestrator"
	"github.com/linkflow-go/internal/execution/app/service"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	config       *config.Config
	logger       logger.Logger
	httpServer   *http.Server
	db           *database.DB
	redis        *redis.Client
	eventBus     events.EventBus
	orchestrator *orchestrator.WorkflowOrchestrator
}

func New(cfg *config.Config, log logger.Logger) (*Server, error) {
	// Initialize database
	db, err := database.New(cfg.Database.ToDatabaseConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize event bus
	eventBus, err := events.NewKafkaEventBus(cfg.Kafka.ToKafkaConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create event bus: %w", err)
	}

	// Initialize repository
	execRepo := repository.NewExecutionRepository(db)

	// Initialize orchestrator
	workflowOrchestrator := orchestrator.NewOrchestrator(
		execRepo, eventBus, redisClient, log,
	)

	// Initialize service
	execService := service.NewExecutionService(
		execRepo, workflowOrchestrator, eventBus, redisClient, log,
	)

	// Initialize handlers
	execHandlers := handlers.NewExecutionHandlers(execService, log)

	// Setup HTTP server
	router := setupRouter(execHandlers, log)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events
	if err := subscribeToEvents(eventBus, execService); err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	if err := eventBus.Subscribe("node.execute.response", workflowOrchestrator.HandleNodeExecuteResponse); err != nil {
		return nil, fmt.Errorf("failed to subscribe to node execute responses: %w", err)
	}

	return &Server{
		config:       cfg,
		logger:       log,
		httpServer:   httpServer,
		db:           db,
		redis:        redisClient,
		eventBus:     eventBus,
		orchestrator: workflowOrchestrator,
	}, nil
}

func setupRouter(h *handlers.ExecutionHandlers, log logger.Logger) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))
	router.Use(metricsMiddleware())

	// Health checks
	router.GET("/health/live", h.Health)
	router.GET("/health/ready", h.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := router.Group("/api/v1/executions")
	{
		v1.GET("", h.ListExecutions)
		v1.POST("", h.StartExecution)
		v1.GET("/:id", h.GetExecution)
		v1.POST("/:id/stop", h.StopExecution)
		v1.POST("/:id/retry", h.RetryExecution)
		v1.DELETE("/:id", h.DeleteExecution)
		v1.GET("/:id/log", h.GetExecutionLog)
		v1.GET("/:id/nodes", h.GetNodeExecutions)
		v1.GET("/stats", h.GetExecutionStats)

		// WebSocket for real-time updates
		v1.GET("/:id/stream", h.StreamExecution)
	}

	// Workflow execution triggers
	triggers := router.Group("/api/v1/trigger")
	{
		triggers.POST("/workflow/:workflowId", h.TriggerWorkflow)
		triggers.POST("/manual/:workflowId", h.ManualTrigger)
		triggers.POST("/test/:workflowId", h.TestExecution)
	}

	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.ExecutionService) error {
	// Subscribe to workflow events
	if err := eventBus.Subscribe("workflow.activated", service.HandleWorkflowActivated); err != nil {
		return err
	}

	if err := eventBus.Subscribe("workflow.deactivated", service.HandleWorkflowDeactivated); err != nil {
		return err
	}

	// Subscribe to trigger events
	if err := eventBus.Subscribe("trigger.fired", service.HandleTriggerFired); err != nil {
		return err
	}

	if err := eventBus.Subscribe("webhook.received", service.HandleWebhookReceived); err != nil {
		return err
	}

	if err := eventBus.Subscribe("schedule.triggered", service.HandleScheduleTriggered); err != nil {
		return err
	}

	return nil
}

func (s *Server) Start() error {
	// Start orchestrator
	go s.orchestrator.Start()

	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Stop orchestrator
	s.orchestrator.Stop()

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	// Close event bus
	if err := s.eventBus.Close(); err != nil {
		s.logger.Error("Failed to close event bus", "error", err)
	}

	// Close Redis
	if err := s.redis.Close(); err != nil {
		s.logger.Error("Failed to close Redis", "error", err)
	}

	// Close database
	if err := s.db.Close(); err != nil {
		s.logger.Error("Failed to close database", "error", err)
	}

	return nil
}

// Middleware functions
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func loggingMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		log.Info("HTTP Request",
			"method", method,
			"path", path,
			"status", statusCode,
			"latency", latency,
			"ip", clientIP,
		)
	}
}

func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := c.Writer.Status()

		// Record metrics
		// This would integrate with Prometheus metrics
		_ = duration
		_ = status
	}
}
