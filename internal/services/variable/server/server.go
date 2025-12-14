package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/domain/variable"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	config     *config.Config
	logger     logger.Logger
	httpServer *http.Server
	db         *database.DB
}

func New(cfg *config.Config, log logger.Logger) (*Server, error) {
	db, err := database.New(cfg.Database.ToDatabaseConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Note: Database migrations are handled via SQL migration files in /migrations
	// Run `make migrate-up` to apply migrations

	router := setupRouter(db, log)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	return &Server{
		config:     cfg,
		logger:     log,
		httpServer: httpServer,
		db:         db,
	}, nil
}

func setupRouter(db *database.DB, log logger.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := router.Group("/api/v1/variables")
	{
		v1.GET("", listVariables(db))
		v1.POST("", createVariable(db))
		v1.GET("/:key", getVariable(db))
		v1.PUT("/:key", updateVariable(db))
		v1.DELETE("/:key", deleteVariable(db))
	}

	return router
}

func listVariables(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var variables []variable.Variable
		if err := db.Find(&variables).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		responses := make([]variable.VariableResponse, len(variables))
		for i, v := range variables {
			responses[i] = v.ToResponse()
		}

		c.JSON(http.StatusOK, gin.H{"data": responses})
	}
}

func createVariable(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Key         string `json:"key" binding:"required"`
			Value       string `json:"value" binding:"required"`
			Type        string `json:"type"`
			Description string `json:"description"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		v := variable.NewVariable(req.Key, req.Value, req.Type)
		v.Description = req.Description

		if err := v.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Create(c.Request.Context(), v); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, v.ToResponse())
	}
}

func getVariable(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")

		var v variable.Variable
		if err := db.Where("key = ?", key).First(&v).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "variable not found"})
			return
		}

		c.JSON(http.StatusOK, v.ToResponse())
	}
}

func updateVariable(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")

		var v variable.Variable
		if err := db.Where("key = ?", key).First(&v).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "variable not found"})
			return
		}

		var req struct {
			Value       string `json:"value"`
			Description string `json:"description"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Value != "" {
			v.Value = req.Value
		}
		if req.Description != "" {
			v.Description = req.Description
		}
		v.UpdatedAt = time.Now()

		if err := db.Save(&v).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, v.ToResponse())
	}
}

func deleteVariable(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")

		if err := db.Where("key = ?", key).Delete(&variable.Variable{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Status(http.StatusNoContent)
	}
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

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	if err := s.db.Close(); err != nil {
		s.logger.Error("Failed to close database", "error", err)
	}

	return nil
}
