package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/notification/channels"
	"github.com/linkflow-go/internal/services/notification/handlers"
	"github.com/linkflow-go/internal/services/notification/repository"
	"github.com/linkflow-go/internal/services/notification/service"
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

	// Initialize notification channels
	// TODO: Add SMTP, Twilio, Slack, FCM config to config.Config
	emailChannel := channels.NewEmailChannel(nil)
	smsChannel := channels.NewSMSChannel(nil)
	slackChannel := channels.NewSlackChannel("")
	pushChannel := channels.NewPushChannel(nil)
	teamsChannel := channels.NewTeamsChannel()
	discordChannel := channels.NewDiscordChannel()

	// Initialize repository
	notificationRepo := repository.NewNotificationRepository(db)

	// Initialize service with all channels
	notificationService := service.NewNotificationService(
		notificationRepo,
		eventBus,
		redisClient,
		log,
		emailChannel,
		smsChannel,
		slackChannel,
		pushChannel,
		teamsChannel,
		discordChannel,
	)

	// Initialize handlers
	notificationHandlers := handlers.NewNotificationHandlers(notificationService, log)

	// Setup HTTP server
	router := setupRouter(notificationHandlers, log)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events for notifications
	if err := subscribeToEvents(eventBus, notificationService); err != nil {
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

func setupRouter(h *handlers.NotificationHandlers, log logger.Logger) *gin.Engine {
	router := gin.New()
	
	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))
	
	// Health checks
	router.GET("/health/live", h.Health)
	router.GET("/health/ready", h.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// API routes
	v1 := router.Group("/api/v1/notifications")
	{
		// Send notifications
		v1.POST("/send", h.SendNotification)
		v1.POST("/send/batch", h.SendBatchNotifications)
		v1.POST("/send/broadcast", h.BroadcastNotification)
		
		// Notification management
		v1.GET("", h.ListNotifications)
		v1.GET("/:id", h.GetNotification)
		v1.PUT("/:id/mark-read", h.MarkAsRead)
		v1.PUT("/mark-all-read", h.MarkAllAsRead)
		v1.DELETE("/:id", h.DeleteNotification)
		
		// Notification templates
		v1.GET("/templates", h.ListTemplates)
		v1.GET("/templates/:id", h.GetTemplate)
		v1.POST("/templates", h.CreateTemplate)
		v1.PUT("/templates/:id", h.UpdateTemplate)
		v1.DELETE("/templates/:id", h.DeleteTemplate)
		
		// User preferences
		v1.GET("/preferences", h.GetPreferences)
		v1.PUT("/preferences", h.UpdatePreferences)
		v1.POST("/preferences/unsubscribe", h.Unsubscribe)
		v1.POST("/preferences/subscribe", h.Subscribe)
		
		// Channel configuration
		v1.GET("/channels", h.ListChannels)
		v1.GET("/channels/:channel/config", h.GetChannelConfig)
		v1.PUT("/channels/:channel/config", h.UpdateChannelConfig)
		v1.POST("/channels/:channel/test", h.TestChannel)
		
		// Notification history
		v1.GET("/history", h.GetHistory)
		v1.GET("/history/stats", h.GetStats)
		
		// Scheduled notifications
		v1.POST("/schedule", h.ScheduleNotification)
		v1.GET("/scheduled", h.ListScheduledNotifications)
		v1.DELETE("/scheduled/:id", h.CancelScheduledNotification)
		
		// Push notifications
		v1.POST("/devices/register", h.RegisterDevice)
		v1.DELETE("/devices/:deviceId", h.UnregisterDevice)
		v1.GET("/devices", h.ListDevices)
	}
	
	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.NotificationService) error {
	// Subscribe to workflow events
	events := []string{
		"workflow.executed",
		"workflow.failed",
		"workflow.error",
		"execution.started",
		"execution.completed",
		"execution.failed",
		"user.registered",
		"user.password_reset",
		"user.invitation",
		"schedule.upcoming",
		"alert.triggered",
		"system.maintenance",
		"billing.payment_failed",
		"billing.subscription_expiring",
	}
	
	for _, event := range events {
		if err := eventBus.Subscribe(event, service.HandleEvent); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", event, err)
		}
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
