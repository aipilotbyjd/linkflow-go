package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/linkflow-go/internal/gateway/server"
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
	log := logger.New(cfg.Logger.ToLoggerConfig())

	// Create server
	srv, err := server.New(cfg, log)
	if err != nil {
		log.Fatal("Failed to create server", "error", err)
	}

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatal("Failed to start server", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("GraphQL gateway exited")
}
