package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/analytics/aggregator"
	"github.com/linkflow-go/internal/services/analytics/handlers"
	"github.com/linkflow-go/internal/services/analytics/repository"
	"github.com/linkflow-go/internal/services/analytics/service"
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
	aggregator *aggregator.MetricsAggregator
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
	analyticsRepo := repository.NewAnalyticsRepository(db)

	// Initialize metrics aggregator
	metricsAggregator := aggregator.NewMetricsAggregator(analyticsRepo, redisClient, log)

	// Initialize service
	analyticsService := service.NewAnalyticsService(analyticsRepo, metricsAggregator, eventBus, log)

	// Initialize handlers
	analyticsHandlers := handlers.NewAnalyticsHandlers(analyticsService, log)

	// Setup HTTP server
	router := setupRouter(analyticsHandlers, log)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events for metrics collection
	if err := subscribeToEvents(eventBus, analyticsService); err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	return &Server{
		config:     cfg,
		logger:     log,
		httpServer: httpServer,
		db:         db,
		redis:      redisClient,
		eventBus:   eventBus,
		aggregator: metricsAggregator,
	}, nil
}

func setupRouter(h *handlers.AnalyticsHandlers, log logger.Logger) *gin.Engine {
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
	v1 := router.Group("/api/v1/analytics")
	{
		// Dashboard endpoints
		v1.GET("/dashboard", h.GetDashboard)
		v1.GET("/dashboard/custom/:id", h.GetCustomDashboard)
		v1.POST("/dashboard/custom", h.CreateCustomDashboard)
		
		// Metrics endpoints
		v1.GET("/metrics/workflows", h.GetWorkflowMetrics)
		v1.GET("/metrics/executions", h.GetExecutionMetrics)
		v1.GET("/metrics/nodes", h.GetNodeMetrics)
		v1.GET("/metrics/users", h.GetUserMetrics)
		v1.GET("/metrics/performance", h.GetPerformanceMetrics)
		
		// Analytics queries
		v1.POST("/query", h.QueryAnalytics)
		v1.GET("/reports", h.ListReports)
		v1.GET("/reports/:id", h.GetReport)
		v1.POST("/reports", h.GenerateReport)
		v1.GET("/reports/:id/export", h.ExportReport)
		
		// Usage analytics
		v1.GET("/usage", h.GetUsageStats)
		v1.GET("/usage/trends", h.GetUsageTrends)
		
		// Anomaly detection
		v1.GET("/anomalies", h.GetAnomalies)
		v1.POST("/anomalies/detect", h.DetectAnomalies)
	}
	
	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.AnalyticsService) error {
	// Subscribe to all execution events for metrics
	events := []string{
		"execution.started",
		"execution.completed",
		"execution.failed",
		"workflow.created",
		"workflow.updated",
		"workflow.deleted",
		"user.registered",
		"user.logged_in",
		"node.execution.started",
		"node.execution.completed",
	}
	
	for _, event := range events {
		if err := eventBus.Subscribe(event, service.ProcessEvent); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", event, err)
		}
	}
	
	return nil
}

func (s *Server) Start() error {
	// Start metrics aggregator
	go s.aggregator.Start(context.Background())
	
	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")
	
	// Stop aggregator
	s.aggregator.Stop()
	
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
