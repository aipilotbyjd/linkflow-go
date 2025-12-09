package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/search/handlers"
	"github.com/linkflow-go/internal/services/search/indexer"
	"github.com/linkflow-go/internal/services/search/service"
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
	esClient   *elasticsearch.Client
	indexer    *indexer.Indexer
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

	// Initialize Elasticsearch client
	esConfig := elasticsearch.Config{
		Addresses: []string{
			"http://localhost:9200",
		},
	}
	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Initialize indexer
	searchIndexer := indexer.NewIndexer(esClient, log)
	
	// Create indices if they don't exist
	if err := searchIndexer.InitializeIndices(); err != nil {
		return nil, fmt.Errorf("failed to initialize indices: %w", err)
	}

	// Initialize service
	searchService := service.NewSearchService(esClient, searchIndexer, eventBus, redisClient, log)

	// Initialize handlers
	searchHandlers := handlers.NewSearchHandlers(searchService, log)

	// Setup HTTP server
	router := setupRouter(searchHandlers, log)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events for indexing
	if err := subscribeToEvents(eventBus, searchService); err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	return &Server{
		config:     cfg,
		logger:     log,
		httpServer: httpServer,
		db:         db,
		redis:      redisClient,
		eventBus:   eventBus,
		esClient:   esClient,
		indexer:    searchIndexer,
	}, nil
}

func setupRouter(h *handlers.SearchHandlers, log logger.Logger) *gin.Engine {
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
	v1 := router.Group("/api/v1/search")
	{
		// Search endpoints
		v1.POST("/", h.Search)
		v1.POST("/advanced", h.AdvancedSearch)
		v1.GET("/suggest", h.Suggest)
		v1.GET("/autocomplete", h.Autocomplete)
		
		// Search by type
		v1.GET("/workflows", h.SearchWorkflows)
		v1.GET("/executions", h.SearchExecutions)
		v1.GET("/nodes", h.SearchNodes)
		v1.GET("/users", h.SearchUsers)
		v1.GET("/audit", h.SearchAuditLogs)
		
		// Faceted search
		v1.POST("/facets", h.FacetedSearch)
		v1.GET("/filters", h.GetAvailableFilters)
		
		// Search templates
		v1.GET("/templates", h.ListSearchTemplates)
		v1.POST("/templates", h.CreateSearchTemplate)
		v1.GET("/templates/:id", h.GetSearchTemplate)
		v1.DELETE("/templates/:id", h.DeleteSearchTemplate)
		
		// Index management
		v1.POST("/index", h.IndexDocument)
		v1.DELETE("/index/:type/:id", h.DeleteDocument)
		v1.POST("/reindex", h.Reindex)
		v1.GET("/stats", h.GetIndexStats)
		
		// Saved searches
		v1.GET("/saved", h.GetSavedSearches)
		v1.POST("/saved", h.SaveSearch)
		v1.DELETE("/saved/:id", h.DeleteSavedSearch)
	}
	
	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.SearchService) error {
	// Subscribe to events for automatic indexing
	events := []string{
		"workflow.created",
		"workflow.updated",
		"workflow.deleted",
		"execution.started",
		"execution.completed",
		"execution.failed",
		"user.registered",
		"user.updated",
		"node.created",
		"node.updated",
	}
	
	for _, event := range events {
		if err := eventBus.Subscribe(event, service.HandleIndexEvent); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", event, err)
		}
	}
	
	return nil
}

func (s *Server) Start() error {
	// Start background indexer
	go s.indexer.StartBackgroundIndexing(context.Background())
	
	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")
	
	// Stop indexer
	s.indexer.Stop()
	
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
