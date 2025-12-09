package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/webhook/handlers"
	"github.com/linkflow-go/internal/services/webhook/repository"
	"github.com/linkflow-go/internal/services/webhook/router"
	"github.com/linkflow-go/internal/services/webhook/service"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	config         *config.Config
	logger         logger.Logger
	httpServer     *http.Server
	db             *database.DB
	redis          *redis.Client
	eventBus       events.EventBus
	webhookRouter  *router.WebhookRouter
}

func New(cfg *config.Config, log logger.Logger) (*Server, error) {
	// Initialize database
	db, err := database.New(cfg.Database)
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
	eventBus, err := events.NewKafkaEventBus(cfg.Kafka)
	if err != nil {
		return nil, fmt.Errorf("failed to create event bus: %w", err)
	}

	// Initialize repository
	webhookRepo := repository.NewWebhookRepository(db)

	// Initialize webhook router
	webhookRouter := router.NewWebhookRouter(redisClient, log)

	// Initialize service
	webhookService := service.NewWebhookService(webhookRepo, webhookRouter, eventBus, redisClient, log)

	// Initialize handlers
	webhookHandlers := handlers.NewWebhookHandlers(webhookService, log)

	// Setup HTTP server
	r := setupRouter(webhookHandlers, webhookRouter, log)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events
	if err := subscribeToEvents(eventBus, webhookService); err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Load webhook routes from database
	if err := webhookRouter.LoadRoutes(context.Background(), webhookRepo); err != nil {
		return nil, fmt.Errorf("failed to load webhook routes: %w", err)
	}

	return &Server{
		config:        cfg,
		logger:        log,
		httpServer:    httpServer,
		db:            db,
		redis:         redisClient,
		eventBus:      eventBus,
		webhookRouter: webhookRouter,
	}, nil
}

func setupRouter(h *handlers.WebhookHandlers, wr *router.WebhookRouter, log logger.Logger) *gin.Engine {
	r := gin.New()
	
	// Middleware
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(loggingMiddleware(log))
	
	// Health checks
	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// Webhook endpoint (dynamic routing)
	r.Any("/webhook/:path", wr.RouteWebhook)
	r.Any("/webhooks/:path", wr.RouteWebhook)
	
	// API routes for webhook management
	v1 := r.Group("/api/v1/webhooks")
	{
		// Webhook CRUD
		v1.GET("", h.ListWebhooks)
		v1.GET("/:id", h.GetWebhook)
		v1.POST("", h.CreateWebhook)
		v1.PUT("/:id", h.UpdateWebhook)
		v1.DELETE("/:id", h.DeleteWebhook)
		
		// Webhook operations
		v1.POST("/:id/enable", h.EnableWebhook)
		v1.POST("/:id/disable", h.DisableWebhook)
		v1.POST("/:id/regenerate-secret", h.RegenerateSecret)
		v1.POST("/:id/test", h.TestWebhook)
		
		// Webhook logs
		v1.GET("/:id/logs", h.GetWebhookLogs)
		v1.GET("/:id/stats", h.GetWebhookStats)
		v1.POST("/:id/retry/:logId", h.RetryWebhook)
		
		// Webhook templates
		v1.GET("/templates", h.ListWebhookTemplates)
		v1.POST("/from-template", h.CreateFromTemplate)
		
		// Verification
		v1.POST("/verify", h.VerifyWebhookSignature)
	}
	
	return r
}

func subscribeToEvents(eventBus events.EventBus, service *service.WebhookService) error {
	// Subscribe to workflow events
	if err := eventBus.Subscribe("workflow.executed", service.HandleWorkflowExecuted); err != nil {
		return err
	}
	
	if err := eventBus.Subscribe("workflow.failed", service.HandleWorkflowFailed); err != nil {
		return err
	}
	
	// Subscribe to execution events
	if err := eventBus.Subscribe("execution.completed", service.HandleExecutionCompleted); err != nil {
		return err
	}
	
	return nil
}

func (s *Server) Start() error {
	// Start webhook router background tasks
	go s.webhookRouter.StartBackgroundTasks(context.Background())
	
	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")
	
	// Stop webhook router
	s.webhookRouter.Stop()
	
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
