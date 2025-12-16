package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/executor/app/worker"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	config     *config.Config
	logger     logger.Logger
	httpServer *http.Server
	pool       *worker.Pool
}

func New(cfg *config.Config, log logger.Logger) (*Server, error) {
	// Create worker pool
	pool, err := worker.NewPool(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker pool: %w", err)
	}

	// Setup HTTP server for health checks
	router := setupRouter(pool, log)

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
		pool:       pool,
	}, nil
}

func setupRouter(pool *worker.Pool, log logger.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	// Health endpoints
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		if pool.Size() > 0 {
			c.JSON(http.StatusOK, gin.H{"status": "ready", "workers": pool.Size()})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
		}
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Worker status
	router.GET("/api/v1/workers/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"workers": pool.Size(),
			"status":  "running",
		})
	})

	return router
}

func (s *Server) Start() error {
	// Start worker pool
	s.logger.Info("Starting worker pool", "workers", s.pool.Size())
	if err := s.pool.Start(); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start HTTP server
	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down executor server...")

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("Failed to shutdown HTTP server", "error", err)
	}

	// Shutdown worker pool
	if err := s.pool.Shutdown(ctx); err != nil {
		s.logger.Error("Failed to shutdown worker pool", "error", err)
	}

	return nil
}
