package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/domain/user"
	"github.com/linkflow-go/internal/services/auth/handlers"
	"github.com/linkflow-go/internal/services/auth/jwt"
	"github.com/linkflow-go/internal/services/auth/rbac"
	"github.com/linkflow-go/internal/services/auth/repository"
	"github.com/linkflow-go/internal/services/auth/service"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/linkflow-go/pkg/middleware/ratelimit"
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

	// Auto-migrate database models
	if err := db.AutoMigrate(
		&user.User{},
		&user.Role{},
		&user.Permission{},
		&user.Session{},
		&user.OAuthToken{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}
	log.Info("Database migration completed")

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

	// Initialize JWT manager
	jwtManager, err := jwt.NewManager(cfg.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	// Initialize RBAC enforcer
	rbacEnforcer, err := rbac.NewEnforcer(db, "configs/rbac_model.conf", "configs/rbac_policy.csv", log)
	if err != nil {
		return nil, fmt.Errorf("failed to create RBAC enforcer: %w", err)
	}

	// Initialize repository
	authRepo := repository.NewAuthRepository(db)

	// Initialize service
	authService := service.NewAuthService(authRepo, jwtManager, redisClient, eventBus, rbacEnforcer, log)

	// Initialize handlers
	authHandlers := handlers.NewAuthHandlers(authService, log)

	// Setup HTTP server
	router := setupRouter(authHandlers, jwtManager, redisClient, log)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
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

func setupRouter(h *handlers.AuthHandlers, jwtManager *jwt.Manager, redisClient *redis.Client, log logger.Logger) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))

	// Health checks
	router.GET("/health", h.Health)
	router.GET("/ready", h.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Create rate limiter for login attempts
	// Allow 5 attempts per 15 minutes, then block for 15 minutes
	var loginRateLimiter ratelimit.RateLimiter
	if redisClient != nil {
		loginRateLimiter = ratelimit.NewRedisRateLimiter(redisClient, 5, 15*time.Minute)
	} else {
		loginRateLimiter = ratelimit.NewInMemoryRateLimiter(5, 15*time.Minute)
	}

	// API routes
	v1 := router.Group("/api/v1/auth")
	{
		// Public routes
		v1.POST("/register", h.Register)
		v1.POST("/login", ratelimit.LoginRateLimitMiddleware(loginRateLimiter), h.Login)
		v1.POST("/refresh", h.RefreshToken)
		v1.POST("/verify-email", h.VerifyEmail)
		v1.POST("/forgot-password", h.ForgotPassword)
		v1.POST("/reset-password", h.ResetPassword)

		// OAuth routes
		v1.GET("/oauth/:provider", h.OAuthLogin)
		v1.GET("/oauth/:provider/callback", h.OAuthCallback)

		// Protected routes
		protected := v1.Group("")
		protected.Use(authMiddleware(jwtManager, redisClient))
		{
			protected.POST("/logout", h.Logout)
			protected.GET("/me", h.GetCurrentUser)
			protected.PUT("/me", h.UpdateProfile)
			protected.PUT("/change-password", h.ChangePassword)
			protected.POST("/2fa/setup", h.Setup2FA)
			protected.POST("/2fa/verify", h.Verify2FA)
			protected.DELETE("/2fa", h.Disable2FA)

			// Session management endpoints
			protected.GET("/sessions", h.GetSessions)
			protected.DELETE("/sessions/:sessionId", h.RevokeSession)
			protected.DELETE("/sessions", h.RevokeAllSessions)
			protected.POST("/validate", h.ValidateToken)

			// RBAC endpoints (admin only)
			rbac := protected.Group("/rbac")
			rbac.Use(RequireRole("admin", "super_admin"))
			{
				rbac.POST("/users/:userId/roles", h.AssignRole)
				rbac.DELETE("/users/:userId/roles/:role", h.RemoveRole)
				rbac.GET("/users/:userId/roles", h.GetUserRoles)
				rbac.GET("/roles", h.GetAllRoles)
				rbac.GET("/roles/:role/users", h.GetUsersForRole)
				rbac.POST("/check-permission", h.CheckPermission)
			}
		}
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
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request details
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

func authMiddleware(jwtManager *jwt.Manager, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// Extract token from Bearer scheme
		const bearerScheme = "Bearer "
		if !strings.HasPrefix(authHeader, bearerScheme) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := authHeader[len(bearerScheme):]

		// Check if token is blacklisted (logged out)
		if redisClient != nil {
			blacklisted, _ := redisClient.Exists(c.Request.Context(), fmt.Sprintf("blacklist:%s", token)).Result()
			if blacklisted > 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
				c.Abort()
				return
			}
		}

		// Validate token
		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Set user context
		c.Set("userId", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("roles", claims.Roles)
		c.Set("permissions", claims.Permissions)
		c.Set("token", token) // Store token for logout

		c.Next()
	}
}

// RequireRole middleware checks if user has any of the required roles
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "no roles found"})
			c.Abort()
			return
		}

		userRolesList, ok := userRoles.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid roles format"})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userRolesList {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions", "required": roles})
			c.Abort()
			return
		}

		c.Next()
	}
}
