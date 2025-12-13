package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/variable/handlers"
	"github.com/linkflow-go/internal/services/variable/repository"
	"github.com/linkflow-go/internal/services/variable/service"
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

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize event bus
	eventBus, err := events.NewKafkaEventBus(cfg.Kafka.ToKafkaConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create event bus: %w", err)
	}

	// Initialize repository and service
	repo := repository.NewRepository(db)
	// Use a default encryption key - in production, this should come from config/secrets
	encryptionKey := "default-32-byte-encryption-key!!"
	svc := service.NewService(repo, eventBus, redisClient, log, encryptionKey)
	h := handlers.NewHandlers(svc, log)

	// Setup router
	router := setupRouter(h, log)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
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

func setupRouter(h *handlers.Handlers, log logger.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))

	// Health checks
	router.GET("/health", h.Health)
	router.GET("/health/live", h.Health)
	router.GET("/health/ready", h.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Variables API - matches n8n's structure
	v1 := router.Group("/api/v1/variables")
	{
		v1.GET("", h.List)
		v1.POST("", h.Create)
		v1.GET("/:id", h.Get)
		v1.PATCH("/:id", h.Update)
		v1.DELETE("/:id", h.Delete)
	}

	return router
}

func (s *Server) Start() error {
	s.logger.Info("Starting Variables service", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down Variables service...")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	if err := s.eventBus.Close(); err != nil {
		s.logger.Error("Failed to close event bus", "error", err)
	}

	if err := s.redis.Close(); err != nil {
		s.logger.Error("Failed to close Redis", "error", err)
	}

	if err := s.db.Close(); err != nil {
		s.logger.Error("Failed to close database", "error", err)
	}

	return nil
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		c.Next()
		log.Info("HTTP Request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start),
		)
	}
}
