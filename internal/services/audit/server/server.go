package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/audit/handlers"
	"github.com/linkflow-go/internal/services/audit/repository"
	"github.com/linkflow-go/internal/services/audit/service"
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
	auditRepo := repository.NewAuditRepository(db)

	// Initialize service
	auditService := service.NewAuditService(auditRepo, eventBus, log)

	// Initialize handlers
	auditHandlers := handlers.NewAuditHandlers(auditService, log)

	// Setup HTTP server
	router := setupRouter(auditHandlers, log)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to ALL events for audit logging
	if err := subscribeToEvents(eventBus, auditService); err != nil {
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

func setupRouter(h *handlers.AuditHandlers, log logger.Logger) *gin.Engine {
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
	v1 := router.Group("/api/v1/audit")
	{
		// Audit log queries
		v1.GET("/logs", h.GetAuditLogs)
		v1.GET("/logs/:id", h.GetAuditLog)
		v1.GET("/logs/user/:userId", h.GetUserAuditLogs)
		v1.GET("/logs/resource/:resourceType/:resourceId", h.GetResourceAuditLogs)
		
		// Compliance reports
		v1.GET("/compliance/report", h.GetComplianceReport)
		v1.GET("/compliance/gdpr", h.GetGDPRReport)
		v1.GET("/compliance/soc2", h.GetSOC2Report)
		v1.GET("/compliance/hipaa", h.GetHIPAAReport)
		
		// Activity tracking
		v1.GET("/activity/timeline", h.GetActivityTimeline)
		v1.GET("/activity/suspicious", h.GetSuspiciousActivity)
		
		// Forensic analysis
		v1.GET("/forensics/investigation/:id", h.GetInvestigation)
		v1.POST("/forensics/investigate", h.StartInvestigation)
		
		// Export and archival
		v1.POST("/export", h.ExportAuditLogs)
		v1.POST("/archive", h.ArchiveOldLogs)
		
		// Search
		v1.POST("/search", h.SearchAuditLogs)
	}
	
	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.AuditService) error {
	// Subscribe to ALL events with wildcard pattern
	// This ensures complete audit trail
	if err := eventBus.Subscribe("*.*", service.LogEvent); err != nil {
		return fmt.Errorf("failed to subscribe to all events: %w", err)
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
