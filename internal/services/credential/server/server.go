package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/credential/handlers"
	"github.com/linkflow-go/internal/services/credential/repository"
	"github.com/linkflow-go/internal/services/credential/service"
	"github.com/linkflow-go/internal/services/credential/vault"
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
	vault      *vault.Vault
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

	// Initialize vault
	credVault, err := vault.NewVault(cfg.Vault.Key, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize vault: %w", err)
	}

	// Initialize repository
	credentialRepo := repository.NewCredentialRepository(db)

	// Initialize service
	credentialService := service.NewCredentialService(credentialRepo, credVault, eventBus, redisClient, log)

	// Initialize handlers
	credentialHandlers := handlers.NewCredentialHandlers(credentialService, log)

	// Setup HTTP server
	router := setupRouter(credentialHandlers, log)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events
	if err := subscribeToEvents(eventBus, credentialService); err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	return &Server{
		config:     cfg,
		logger:     log,
		httpServer: httpServer,
		db:         db,
		redis:      redisClient,
		eventBus:   eventBus,
		vault:      credVault,
	}, nil
}

func setupRouter(h *handlers.CredentialHandlers, log logger.Logger) *gin.Engine {
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
	v1 := router.Group("/api/v1/credentials")
	{
		// Credential CRUD
		v1.GET("", h.ListCredentials)
		v1.GET("/:id", h.GetCredential)
		v1.POST("", h.CreateCredential)
		v1.PUT("/:id", h.UpdateCredential)
		v1.DELETE("/:id", h.DeleteCredential)
		
		// Credential operations
		v1.POST("/:id/test", h.TestCredential)
		v1.POST("/:id/rotate", h.RotateCredential)
		v1.GET("/:id/decrypt", h.DecryptCredential)
		v1.POST("/:id/share", h.ShareCredential)
		v1.DELETE("/:id/share/:userId", h.UnshareCredential)
		
		// Credential types
		v1.GET("/types", h.ListCredentialTypes)
		v1.GET("/types/:type/schema", h.GetCredentialTypeSchema)
		
		// OAuth operations
		v1.GET("/oauth/:provider/authorize", h.OAuthAuthorize)
		v1.GET("/oauth/:provider/callback", h.OAuthCallback)
		v1.POST("/oauth/:provider/refresh", h.OAuthRefresh)
		v1.DELETE("/oauth/:provider/revoke", h.OAuthRevoke)
		
		// API key management
		v1.POST("/api-keys", h.CreateAPIKey)
		v1.GET("/api-keys", h.ListAPIKeys)
		v1.DELETE("/api-keys/:id", h.RevokeAPIKey)
		v1.POST("/api-keys/:id/regenerate", h.RegenerateAPIKey)
		
		// SSH key management
		v1.POST("/ssh-keys", h.CreateSSHKey)
		v1.GET("/ssh-keys", h.ListSSHKeys)
		v1.DELETE("/ssh-keys/:id", h.DeleteSSHKey)
		v1.GET("/ssh-keys/:id/public", h.GetPublicKey)
		
		// Certificate management
		v1.POST("/certificates", h.UploadCertificate)
		v1.GET("/certificates", h.ListCertificates)
		v1.GET("/certificates/:id", h.GetCertificate)
		v1.DELETE("/certificates/:id", h.DeleteCertificate)
		v1.GET("/certificates/:id/verify", h.VerifyCertificate)
		
		// Vault operations
		v1.POST("/vault/backup", h.BackupVault)
		v1.POST("/vault/restore", h.RestoreVault)
		v1.POST("/vault/rekey", h.RekeyVault)
		v1.GET("/vault/status", h.GetVaultStatus)
		
		// Audit
		v1.GET("/:id/audit", h.GetCredentialAudit)
		v1.GET("/audit", h.GetAuditLogs)
		
		// Import/Export
		v1.POST("/import", h.ImportCredentials)
		v1.GET("/export", h.ExportCredentials)
	}
	
	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.CredentialService) error {
	// Subscribe to credential-related events
	if err := eventBus.Subscribe("credential.expiring", service.HandleCredentialExpiring); err != nil {
		return err
	}
	
	if err := eventBus.Subscribe("credential.expired", service.HandleCredentialExpired); err != nil {
		return err
	}
	
	if err := eventBus.Subscribe("oauth.token_expired", service.HandleOAuthTokenExpired); err != nil {
		return err
	}
	
	// Subscribe to security events
	if err := eventBus.Subscribe("security.breach_detected", service.HandleSecurityBreach); err != nil {
		return err
	}
	
	return nil
}

func (s *Server) Start() error {
	// Start background tasks
	go s.startBackgroundTasks()
	
	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Server) startBackgroundTasks() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check for expiring credentials
			s.logger.Info("Checking for expiring credentials")
			// Implementation would go here
		}
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")
	
	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}
	
	// Close vault
	if err := s.vault.Close(); err != nil {
		s.logger.Error("Failed to close vault", "error", err)
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
