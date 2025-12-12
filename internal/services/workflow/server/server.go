package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/workflow/handlers"
	"github.com/linkflow-go/internal/services/workflow/repository"
	"github.com/linkflow-go/internal/services/workflow/service"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	config     *config.Config
	logger     logger.Logger
	httpServer *http.Server
	db         *database.DB
	redis      *redis.Client
	eventBus   events.EventBus
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
	workflowRepo := repository.NewWorkflowRepository(db)

	// Initialize service
	workflowService := service.NewWorkflowService(workflowRepo, eventBus, redisClient, log, db)

	// Initialize handlers
	workflowHandlers := handlers.NewWorkflowHandlers(workflowService, log)

	// Setup HTTP server
	router := setupRouter(workflowHandlers, log)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events
	if err := subscribeToEvents(eventBus, workflowService); err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	return &Server{
		config:     cfg,
		logger:     log,
		httpServer: httpServer,
		db:         db,
		redis:      redisClient,
		eventBus:   eventBus,
	}, nil
}

func setupRouter(h *handlers.WorkflowHandlers, log logger.Logger) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))

	// Health checks
	router.GET("/health", h.Health)
	router.GET("/ready", h.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := router.Group("/api/v1/workflows")
	{
		// Workflow CRUD
		v1.GET("", h.ListWorkflows)
		v1.GET("/:id", h.GetWorkflow)
		v1.POST("", h.CreateWorkflow)
		v1.PUT("/:id", h.UpdateWorkflow)
		v1.DELETE("/:id", h.DeleteWorkflow)

		// Workflow versions
		v1.GET("/:id/versions", h.GetWorkflowVersions)
		v1.GET("/:id/versions/:version", h.GetWorkflowVersion)
		v1.POST("/:id/versions", h.CreateWorkflowVersion)
		v1.POST("/:id/rollback/:version", h.RollbackWorkflowVersion)

		// Workflow operations
		v1.POST("/:id/activate", h.ActivateWorkflow)
		v1.POST("/:id/deactivate", h.DeactivateWorkflow)
		v1.POST("/:id/duplicate", h.DuplicateWorkflow)
		v1.POST("/:id/validate", h.ValidateWorkflow)
		v1.POST("/:id/execute", h.ExecuteWorkflow)
		v1.POST("/:id/test", h.TestWorkflow)

		// Workflow sharing
		v1.GET("/:id/permissions", h.GetWorkflowPermissions)
		v1.POST("/:id/share", h.ShareWorkflow)
		v1.DELETE("/:id/share/:userId", h.UnshareWorkflow)
		v1.POST("/:id/publish", h.PublishWorkflow)

		// Workflow templates
		v1.GET("/templates", h.ListTemplates)
		v1.GET("/templates/:id", h.GetTemplate)
		v1.POST("/templates", h.CreateTemplate)
		v1.POST("/from-template/:templateId", h.CreateFromTemplate)

		// Workflow import/export
		v1.POST("/import", h.ImportWorkflow)
		v1.GET("/:id/export", h.ExportWorkflow)

		// Workflow statistics
		v1.GET("/:id/stats", h.GetWorkflowStats)
		v1.GET("/:id/executions", h.GetWorkflowExecutions)
		v1.GET("/:id/runs/latest", h.GetLatestRun)

		// Workflow categories
		v1.GET("/categories", h.ListCategories)
		v1.POST("/categories", h.CreateCategory)

		// Search and filter
		v1.GET("/search", h.SearchWorkflows)
		v1.GET("/tags", h.GetPopularTags)
	}

	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.WorkflowService) error {
	// Subscribe to execution events for stats
	if err := eventBus.Subscribe("execution.completed", service.HandleExecutionCompleted); err != nil {
		return err
	}

	if err := eventBus.Subscribe("execution.failed", service.HandleExecutionFailed); err != nil {
		return err
	}

	// Subscribe to node events for workflow validation
	if err := eventBus.Subscribe("node.updated", service.HandleNodeUpdated); err != nil {
		return err
	}

	return nil
}

func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

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
