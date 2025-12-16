package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/user/adapters/db/repository"
	"github.com/linkflow-go/internal/user/adapters/http/handlers"
	"github.com/linkflow-go/internal/user/app/service"
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
	userRepo := repository.NewUserRepository(db)

	// Initialize service
	userService := service.NewUserService(userRepo, eventBus, redisClient, log)

	// Initialize handlers
	userHandlers := handlers.NewUserHandlers(userService, log)

	// Setup HTTP server
	router := setupRouter(userHandlers, log)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events
	if err := subscribeToEvents(eventBus, userService); err != nil {
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

func setupRouter(h *handlers.UserHandlers, log logger.Logger) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))

	// Health checks
	router.GET("/health/live", h.Health)
	router.GET("/health/ready", h.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// OpenAPI/Swagger documentation
	router.GET("/api/docs", serveSwaggerUI())
	router.StaticFile("/api/openapi.yaml", "api/openapi/user.yaml")

	// API routes
	v1 := router.Group("/api/v1/users")
	{
		v1.GET("", h.ListUsers)
		v1.GET("/:id", h.GetUser)
		v1.PUT("/:id", h.UpdateUser)
		v1.DELETE("/:id", h.DeleteUser)
		v1.GET("/:id/permissions", h.GetUserPermissions)

		// Team management
		v1.POST("/teams", h.CreateTeam)
		v1.GET("/teams", h.ListTeams)
		v1.GET("/teams/:id", h.GetTeam)
		v1.PUT("/teams/:id", h.UpdateTeam)
		v1.DELETE("/teams/:id", h.DeleteTeam)
		v1.POST("/teams/:id/members", h.AddTeamMember)
		v1.DELETE("/teams/:id/members/:userId", h.RemoveTeamMember)

		// Role management
		v1.GET("/roles", h.ListRoles)
		v1.POST("/roles", h.CreateRole)
		v1.PUT("/roles/:id", h.UpdateRole)
		v1.DELETE("/roles/:id", h.DeleteRole)
		v1.POST("/roles/:id/assign", h.AssignRole)
		v1.DELETE("/roles/:id/revoke", h.RevokeRole)

		// Permission management
		v1.GET("/permissions", h.ListPermissions)
		v1.POST("/permissions", h.CreatePermission)
		v1.DELETE("/permissions/:id", h.DeletePermission)
	}

	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.UserService) error {
	// Subscribe to auth events
	if err := eventBus.Subscribe("user.registered", service.HandleUserRegistered); err != nil {
		return err
	}

	if err := eventBus.Subscribe("user.deleted", service.HandleUserDeleted); err != nil {
		return err
	}

	// Subscribe to workflow events for ownership tracking
	if err := eventBus.Subscribe("workflow.created", service.HandleWorkflowCreated); err != nil {
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

// serveSwaggerUI returns a handler that serves Swagger UI
func serveSwaggerUI() gin.HandlerFunc {
	return func(c *gin.Context) {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LinkFlow User API - Swagger UI</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
    <style>body { margin: 0; padding: 0; } .topbar { display: none; }</style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/api/openapi.yaml",
                dom_id: '#swagger-ui',
                presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
                layout: "BaseLayout",
                deepLinking: true,
                persistAuthorization: true
            });
        };
    </script>
</body>
</html>`
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
	}
}
