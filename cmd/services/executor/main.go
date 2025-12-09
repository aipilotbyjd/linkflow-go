package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/linkflow-go/internal/services/executor/worker"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load("executor-service")
	if err != nil {
		panic(err)
	}

	// Initialize logger
	log := logger.New(cfg.Logger.ToLoggerConfig())

	// Create worker pool
	pool, err := worker.NewPool(cfg, log)
	if err != nil {
		log.Fatal("Failed to create worker pool", "error", err)
	}

	// Start workers
	log.Info("Starting executor workers", "workers", pool.Size())
	if err := pool.Start(); err != nil {
		log.Fatal("Failed to start worker pool", "error", err)
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down executor service...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pool.Shutdown(ctx); err != nil {
		log.Error("Worker pool forced to shutdown", "error", err)
	}

	log.Info("Executor service exited")
}
