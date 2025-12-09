package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/storage/handlers"
	"github.com/linkflow-go/internal/services/storage/repository"
	"github.com/linkflow-go/internal/services/storage/service"
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
	s3Client   *s3.S3
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

	// Initialize S3 client (or MinIO)
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String("us-east-1"),
		Endpoint: aws.String("http://localhost:9000"), // MinIO endpoint for local dev
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 session: %w", err)
	}
	s3Client := s3.New(sess)

	// Initialize repository
	storageRepo := repository.NewStorageRepository(db)

	// Initialize service
	storageService := service.NewStorageService(storageRepo, s3Client, eventBus, redisClient, log)

	// Initialize handlers
	storageHandlers := handlers.NewStorageHandlers(storageService, log)

	// Setup HTTP server
	router := setupRouter(storageHandlers, log)
	
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
		s3Client:   s3Client,
	}, nil
}

func setupRouter(h *handlers.StorageHandlers, log logger.Logger) *gin.Engine {
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
	v1 := router.Group("/api/v1/storage")
	{
		// File operations
		v1.POST("/upload", h.UploadFile)
		v1.POST("/upload/multipart", h.MultipartUpload)
		v1.GET("/files/:id", h.GetFile)
		v1.DELETE("/files/:id", h.DeleteFile)
		v1.GET("/files/:id/metadata", h.GetFileMetadata)
		v1.PUT("/files/:id/metadata", h.UpdateFileMetadata)
		
		// Direct upload URLs
		v1.POST("/presigned-url", h.GetPresignedURL)
		v1.POST("/presigned-url/upload", h.GetUploadPresignedURL)
		v1.POST("/presigned-url/download", h.GetDownloadPresignedURL)
		
		// Folder operations
		v1.GET("/folders", h.ListFolders)
		v1.POST("/folders", h.CreateFolder)
		v1.DELETE("/folders/:path", h.DeleteFolder)
		v1.GET("/folders/:path/files", h.ListFolderFiles)
		
		// File management
		v1.POST("/files/:id/copy", h.CopyFile)
		v1.POST("/files/:id/move", h.MoveFile)
		v1.POST("/files/:id/share", h.ShareFile)
		v1.GET("/files/:id/versions", h.GetFileVersions)
		
		// Image processing
		v1.POST("/images/resize", h.ResizeImage)
		v1.POST("/images/thumbnail", h.GenerateThumbnail)
		
		// Quota management
		v1.GET("/quota", h.GetQuota)
		v1.GET("/usage", h.GetUsage)
		
		// Export/Import
		v1.POST("/export", h.ExportFiles)
		v1.POST("/import", h.ImportFiles)
		
		// Search
		v1.GET("/search", h.SearchFiles)
	}
	
	return router
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
