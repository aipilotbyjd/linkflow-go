package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/billing/handlers"
	"github.com/linkflow-go/internal/services/billing/repository"
	"github.com/linkflow-go/internal/services/billing/service"
	"github.com/linkflow-go/internal/services/billing/stripe"
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

	// Initialize Stripe client
	// TODO: Add Stripe config to config.Config
	stripeClient := stripe.NewClient("")

	// Initialize repository
	billingRepo := repository.NewBillingRepository(db)

	// Initialize service
	billingService := service.NewBillingService(billingRepo, stripeClient, eventBus, redisClient, log)

	// Initialize handlers
	billingHandlers := handlers.NewBillingHandlers(billingService, log)

	// Setup HTTP server
	router := setupRouter(billingHandlers, log)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Subscribe to events for billing
	if err := subscribeToEvents(eventBus, billingService); err != nil {
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

func setupRouter(h *handlers.BillingHandlers, log logger.Logger) *gin.Engine {
	router := gin.New()
	
	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))
	
	// Health checks
	router.GET("/health", h.Health)
	router.GET("/ready", h.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// Webhook endpoints (no auth)
	router.POST("/webhooks/stripe", h.HandleStripeWebhook)
	
	// API routes
	v1 := router.Group("/api/v1/billing")
	{
		// Subscription management
		v1.GET("/subscriptions", h.GetSubscriptions)
		v1.GET("/subscriptions/:id", h.GetSubscription)
		v1.POST("/subscriptions", h.CreateSubscription)
		v1.PUT("/subscriptions/:id", h.UpdateSubscription)
		v1.DELETE("/subscriptions/:id", h.CancelSubscription)
		v1.POST("/subscriptions/:id/reactivate", h.ReactivateSubscription)
		
		// Plans
		v1.GET("/plans", h.ListPlans)
		v1.GET("/plans/:id", h.GetPlan)
		v1.POST("/plans", h.CreatePlan)
		v1.PUT("/plans/:id", h.UpdatePlan)
		v1.DELETE("/plans/:id", h.DeletePlan)
		
		// Payment methods
		v1.GET("/payment-methods", h.ListPaymentMethods)
		v1.POST("/payment-methods", h.AddPaymentMethod)
		v1.DELETE("/payment-methods/:id", h.RemovePaymentMethod)
		v1.POST("/payment-methods/:id/default", h.SetDefaultPaymentMethod)
		
		// Invoices
		v1.GET("/invoices", h.ListInvoices)
		v1.GET("/invoices/:id", h.GetInvoice)
		v1.GET("/invoices/:id/download", h.DownloadInvoice)
		v1.POST("/invoices/:id/pay", h.PayInvoice)
		
		// Usage and metering
		v1.GET("/usage", h.GetUsage)
		v1.POST("/usage/report", h.ReportUsage)
		v1.GET("/usage/summary", h.GetUsageSummary)
		
		// Billing info
		v1.GET("/info", h.GetBillingInfo)
		v1.PUT("/info", h.UpdateBillingInfo)
		
		// Credits and promotions
		v1.GET("/credits", h.GetCredits)
		v1.POST("/credits/apply", h.ApplyPromoCode)
		v1.GET("/promotions", h.GetAvailablePromotions)
		
		// Billing portal
		v1.POST("/portal/session", h.CreatePortalSession)
		
		// Reports
		v1.GET("/reports/revenue", h.GetRevenueReport)
		v1.GET("/reports/churn", h.GetChurnReport)
		v1.GET("/reports/mrr", h.GetMRRReport)
	}
	
	return router
}

func subscribeToEvents(eventBus events.EventBus, service *service.BillingService) error {
	// Subscribe to events that affect billing
	events := []string{
		"user.registered",
		"user.deleted",
		"execution.completed",
		"workflow.created",
		"storage.used",
		"subscription.trial_ending",
	}
	
	for _, event := range events {
		if err := eventBus.Subscribe(event, service.HandleBillingEvent); err != nil {
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
