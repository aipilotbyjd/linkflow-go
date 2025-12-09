package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/gateway/graph"
	"github.com/linkflow-go/internal/gateway/graph/generated"
	"github.com/linkflow-go/internal/gateway/resolver"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load("graphql-gateway")
	if err != nil {
		panic(err)
	}

	// Initialize logger
	log := logger.New(cfg.Logger)

	// Create resolver
	resolver := resolver.NewResolver(cfg, log)

	// Create GraphQL server
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))

	// Setup Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// GraphQL endpoint
	router.POST("/graphql", graphqlHandler(srv))
	router.GET("/graphql", graphqlHandler(srv))

	// GraphQL playground
	router.GET("/playground", playgroundHandler())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    ":4000",
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Info("Starting GraphQL gateway", "port", 4000)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down GraphQL gateway...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("GraphQL gateway exited")
}

func graphqlHandler(srv *handler.Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		srv.ServeHTTP(c.Writer, c.Request)
	}
}

func playgroundHandler() gin.HandlerFunc {
	h := playground.Handler("GraphQL Playground", "/graphql")
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

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
