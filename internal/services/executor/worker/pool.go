package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type Pool struct {
	config   *config.Config
	logger   logger.Logger
	workers  []*Worker
	eventBus events.EventBus
	redis    *redis.Client
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

type Worker struct {
	id       int
	pool     *Pool
	executor *NodeExecutor
	stopCh   chan struct{}
}

func NewPool(cfg *config.Config, log logger.Logger) (*Pool, error) {
	// Initialize event bus
	eventBus, err := events.NewKafkaEventBus(cfg.Kafka.ToKafkaConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create event bus: %w", err)
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

	// Determine number of workers (default to number of CPUs)
	numWorkers := runtime.NumCPU()
	if cfg.Server.Port > 0 { // Use port as worker count if specified
		numWorkers = cfg.Server.Port / 1000 // Simple calculation
	}
	if numWorkers < 2 {
		numWorkers = 2
	}
	if numWorkers > 100 {
		numWorkers = 100
	}

	pool := &Pool{
		config:   cfg,
		logger:   log,
		workers:  make([]*Worker, numWorkers),
		eventBus: eventBus,
		redis:    redisClient,
		stopCh:   make(chan struct{}),
	}

	// Create workers
	for i := 0; i < numWorkers; i++ {
		worker := &Worker{
			id:       i + 1,
			pool:     pool,
			executor: NewNodeExecutor(eventBus, redisClient, log),
			stopCh:   make(chan struct{}),
		}
		pool.workers[i] = worker
	}

	return pool, nil
}

func (p *Pool) Size() int {
	return len(p.workers)
}

func (p *Pool) Start() error {
	// Subscribe to node execution requests
	if err := p.eventBus.Subscribe("node.execute.request", p.handleNodeExecutionRequest); err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Start all workers
	for _, worker := range p.workers {
		p.wg.Add(1)
		go worker.run()
	}

	// Start monitoring
	go p.monitor()

	p.logger.Info("Worker pool started", "workers", len(p.workers))
	return nil
}

func (p *Pool) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down worker pool...")
	
	// Signal all workers to stop
	close(p.stopCh)
	
	// Stop all workers
	for _, worker := range p.workers {
		close(worker.stopCh)
	}
	
	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		p.logger.Info("All workers stopped gracefully")
	case <-ctx.Done():
		p.logger.Warn("Timeout waiting for workers to stop")
	}
	
	// Close connections
	if err := p.eventBus.Close(); err != nil {
		p.logger.Error("Failed to close event bus", "error", err)
	}
	
	if err := p.redis.Close(); err != nil {
		p.logger.Error("Failed to close Redis", "error", err)
	}
	
	return nil
}

func (p *Pool) handleNodeExecutionRequest(ctx context.Context, event events.Event) error {
	// Find available worker and assign task
	// In production, this would use a proper work queue
	
	// For now, just log the request
	p.logger.Info("Received node execution request",
		"nodeId", event.Payload["nodeId"],
		"nodeType", event.Payload["nodeType"],
	)
	
	// Execute node (simplified)
	result := map[string]interface{}{
		"status": "completed",
		"output": "Node executed successfully",
	}
	
	// Publish result
	responseEvent := events.NewEventBuilder("node.execute.response").
		WithAggregateID(event.AggregateID).
		WithPayload("nodeId", event.Payload["nodeId"]).
		WithPayload("result", result).
		Build()
	
	return p.eventBus.Publish(ctx, responseEvent)
}

func (w *Worker) run() {
	defer w.pool.wg.Done()
	
	w.pool.logger.Info("Worker started", "workerId", w.id)
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check for work (simplified - in production would use proper queue)
			w.checkForWork()
		case <-w.stopCh:
			w.pool.logger.Info("Worker stopped", "workerId", w.id)
			return
		case <-w.pool.stopCh:
			w.pool.logger.Info("Worker stopped by pool", "workerId", w.id)
			return
		}
	}
}

func (w *Worker) checkForWork() {
	// In production, this would:
	// 1. Pull work from a queue (Redis, RabbitMQ, etc.)
	// 2. Execute the node
	// 3. Report results back
	
	// For now, just heartbeat
	w.pool.logger.Debug("Worker checking for work", "workerId", w.id)
}

func (p *Pool) monitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.reportMetrics()
		case <-p.stopCh:
			return
		}
	}
}

func (p *Pool) reportMetrics() {
	// Report worker pool metrics
	activeWorkers := 0
	for _, worker := range p.workers {
		select {
		case <-worker.stopCh:
			// Worker is stopped
		default:
			activeWorkers++
		}
	}
	
	p.logger.Info("Worker pool metrics",
		"totalWorkers", len(p.workers),
		"activeWorkers", activeWorkers,
	)
	
	// In production, this would send metrics to Prometheus
}
