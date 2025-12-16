package queue

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// WorkerPool manages a pool of workers for executing tasks
type WorkerPool struct {
	mu             sync.RWMutex
	workers        []*Worker
	taskQueue      chan *WorkerTask
	resultQueue    chan *WorkerResult
	maxWorkers     int
	minWorkers     int
	activeWorkers  int32
	totalTasks     int64
	completedTasks int64
	failedTasks    int64
	executor       ExecutorFunc
	logger         logger.Logger
	eventBus       events.EventBus

	// Dynamic scaling
	scaleUpThreshold   float64
	scaleDownThreshold float64
	scalingEnabled     bool
	lastScaleTime      time.Time
	scaleCooldown      time.Duration

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// Worker represents a single worker in the pool
type Worker struct {
	id             int
	pool           *WorkerPool
	taskQueue      chan *WorkerTask
	stopCh         chan struct{}
	isActive       atomic.Bool
	lastActive     time.Time
	tasksProcessed int64
}

// WorkerTask represents a task to be executed by a worker
type WorkerTask struct {
	ID         string                     `json:"id"`
	WorkflowID string                     `json:"workflowId"`
	Request    *workflow.ExecutionRequest `json:"request"`
	CreatedAt  time.Time                  `json:"createdAt"`
	Timeout    time.Duration              `json:"timeout"`
	Retries    int                        `json:"retries"`
}

// WorkerResult represents the result of a task execution
type WorkerResult struct {
	TaskID      string                 `json:"taskId"`
	Success     bool                   `json:"success"`
	Error       error                  `json:"error,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	WorkerID    int                    `json:"workerId"`
	Duration    time.Duration          `json:"duration"`
	CompletedAt time.Time              `json:"completedAt"`
}

// ExecutorFunc is the function that executes tasks
type ExecutorFunc func(ctx context.Context, task *WorkerTask) (*WorkerResult, error)

// WorkerPoolConfig contains configuration for the worker pool
type WorkerPoolConfig struct {
	MinWorkers         int
	MaxWorkers         int
	TaskQueueSize      int
	EnableScaling      bool
	ScaleUpThreshold   float64 // CPU usage percentage
	ScaleDownThreshold float64
	ScaleCooldown      time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	config WorkerPoolConfig,
	executor ExecutorFunc,
	eventBus events.EventBus,
	logger logger.Logger,
) *WorkerPool {
	if config.MinWorkers == 0 {
		config.MinWorkers = 2
	}
	if config.MaxWorkers == 0 {
		config.MaxWorkers = runtime.NumCPU() * 2
	}
	if config.TaskQueueSize == 0 {
		config.TaskQueueSize = 1000
	}
	if config.ScaleUpThreshold == 0 {
		config.ScaleUpThreshold = 0.8
	}
	if config.ScaleDownThreshold == 0 {
		config.ScaleDownThreshold = 0.2
	}
	if config.ScaleCooldown == 0 {
		config.ScaleCooldown = 30 * time.Second
	}

	pool := &WorkerPool{
		workers:            make([]*Worker, 0, config.MaxWorkers),
		taskQueue:          make(chan *WorkerTask, config.TaskQueueSize),
		resultQueue:        make(chan *WorkerResult, 100),
		maxWorkers:         config.MaxWorkers,
		minWorkers:         config.MinWorkers,
		executor:           executor,
		logger:             logger,
		eventBus:           eventBus,
		scaleUpThreshold:   config.ScaleUpThreshold,
		scaleDownThreshold: config.ScaleDownThreshold,
		scalingEnabled:     config.EnableScaling,
		scaleCooldown:      config.ScaleCooldown,
		stopCh:             make(chan struct{}),
	}

	return pool
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.logger.Info("Starting worker pool", "minWorkers", wp.minWorkers, "maxWorkers", wp.maxWorkers)

	// Start minimum number of workers
	for i := 0; i < wp.minWorkers; i++ {
		if err := wp.addWorker(ctx); err != nil {
			return fmt.Errorf("failed to start worker %d: %w", i, err)
		}
	}

	// Start result processor
	go wp.processResults(ctx)

	// Start autoscaler if enabled
	if wp.scalingEnabled {
		go wp.autoscale(ctx)
	}

	// Start metrics reporter
	go wp.reportMetrics(ctx)

	return nil
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop(ctx context.Context) error {
	wp.logger.Info("Stopping worker pool")

	// Close stop channel
	close(wp.stopCh)

	// Close task queue
	close(wp.taskQueue)

	// Stop all workers
	wp.mu.Lock()
	for _, worker := range wp.workers {
		close(worker.stopCh)
	}
	wp.mu.Unlock()

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wp.logger.Info("All workers stopped")
	case <-ctx.Done():
		wp.logger.Warn("Timeout waiting for workers to stop")
	}

	// Close result queue
	close(wp.resultQueue)

	return nil
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(ctx context.Context, task *WorkerTask) error {
	if task.Timeout == 0 {
		task.Timeout = 5 * time.Minute
	}

	select {
	case wp.taskQueue <- task:
		atomic.AddInt64(&wp.totalTasks, 1)
		wp.logger.Debug("Task submitted", "taskId", task.ID)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while submitting task")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout submitting task - queue may be full")
	}
}

// HasAvailableWorker checks if there's an available worker
func (wp *WorkerPool) HasAvailableWorker() bool {
	return len(wp.taskQueue) < cap(wp.taskQueue)/2
}

// ActiveWorkers returns the number of active workers
func (wp *WorkerPool) ActiveWorkers() int {
	return int(atomic.LoadInt32(&wp.activeWorkers))
}

// TotalWorkers returns the total number of workers
func (wp *WorkerPool) TotalWorkers() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return len(wp.workers)
}

// addWorker adds a new worker to the pool
func (wp *WorkerPool) addWorker(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if len(wp.workers) >= wp.maxWorkers {
		return fmt.Errorf("worker pool is at maximum capacity")
	}

	workerID := len(wp.workers) + 1
	worker := &Worker{
		id:        workerID,
		pool:      wp,
		taskQueue: wp.taskQueue,
		stopCh:    make(chan struct{}),
	}

	wp.workers = append(wp.workers, worker)

	// Start worker
	wp.wg.Add(1)
	go worker.run(ctx)

	wp.logger.Info("Added worker to pool", "workerId", workerID, "totalWorkers", len(wp.workers))

	return nil
}

// removeWorker removes a worker from the pool
func (wp *WorkerPool) removeWorker() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if len(wp.workers) <= wp.minWorkers {
		return fmt.Errorf("worker pool is at minimum capacity")
	}

	// Find idle worker to remove
	var workerToRemove *Worker
	for _, worker := range wp.workers {
		if !worker.isActive.Load() {
			workerToRemove = worker
			break
		}
	}

	if workerToRemove == nil {
		return fmt.Errorf("no idle workers to remove")
	}

	// Signal worker to stop
	close(workerToRemove.stopCh)

	// Remove from workers slice
	newWorkers := make([]*Worker, 0, len(wp.workers)-1)
	for _, w := range wp.workers {
		if w.id != workerToRemove.id {
			newWorkers = append(newWorkers, w)
		}
	}
	wp.workers = newWorkers

	wp.logger.Info("Removed worker from pool", "workerId", workerToRemove.id, "totalWorkers", len(wp.workers))

	return nil
}

// Worker methods

func (w *Worker) run(ctx context.Context) {
	defer w.pool.wg.Done()

	w.pool.logger.Info("Worker started", "workerId", w.id)

	for {
		select {
		case task, ok := <-w.taskQueue:
			if !ok {
				w.pool.logger.Info("Task queue closed, worker stopping", "workerId", w.id)
				return
			}

			w.processTask(ctx, task)

		case <-w.stopCh:
			w.pool.logger.Info("Worker stopped", "workerId", w.id)
			return

		case <-w.pool.stopCh:
			w.pool.logger.Info("Worker stopped by pool", "workerId", w.id)
			return
		}
	}
}

func (w *Worker) processTask(ctx context.Context, task *WorkerTask) {
	// Mark worker as active
	w.isActive.Store(true)
	atomic.AddInt32(&w.pool.activeWorkers, 1)
	defer func() {
		w.isActive.Store(false)
		atomic.AddInt32(&w.pool.activeWorkers, -1)
		w.lastActive = time.Now()
		atomic.AddInt64(&w.tasksProcessed, 1)
	}()

	startTime := time.Now()

	// Create task context with timeout
	taskCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	// Execute task
	result, err := w.pool.executor(taskCtx, task)

	// Create result
	if result == nil {
		result = &WorkerResult{
			TaskID:   task.ID,
			WorkerID: w.id,
		}
	}

	result.Duration = time.Since(startTime)
	result.CompletedAt = time.Now()

	if err != nil {
		result.Success = false
		result.Error = err
		atomic.AddInt64(&w.pool.failedTasks, 1)

		w.pool.logger.Error("Task execution failed",
			"taskId", task.ID,
			"workerId", w.id,
			"error", err,
			"duration", result.Duration,
		)
	} else {
		result.Success = true
		atomic.AddInt64(&w.pool.completedTasks, 1)

		w.pool.logger.Info("Task execution completed",
			"taskId", task.ID,
			"workerId", w.id,
			"duration", result.Duration,
		)
	}

	// Send result to result queue
	select {
	case w.pool.resultQueue <- result:
	case <-time.After(5 * time.Second):
		w.pool.logger.Error("Failed to send result - result queue full", "taskId", task.ID)
	}
}

// processResults processes task results
func (wp *WorkerPool) processResults(ctx context.Context) {
	for {
		select {
		case result, ok := <-wp.resultQueue:
			if !ok {
				return
			}

			// Publish result event
			event := events.NewEventBuilder("task.completed").
				WithAggregateID(result.TaskID).
				WithPayload("success", result.Success).
				WithPayload("duration", result.Duration).
				WithPayload("workerId", result.WorkerID).
				Build()

			if result.Error != nil {
				event.Payload["error"] = result.Error.Error()
			}

			if err := wp.eventBus.Publish(ctx, event); err != nil {
				wp.logger.Error("Failed to publish task result", "error", err, "taskId", result.TaskID)
			}

		case <-wp.stopCh:
			return
		}
	}
}

// autoscale automatically scales the worker pool based on load
func (wp *WorkerPool) autoscale(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.checkScaling(ctx)
		case <-wp.stopCh:
			return
		}
	}
}

func (wp *WorkerPool) checkScaling(ctx context.Context) {
	// Check if cooling down
	if time.Since(wp.lastScaleTime) < wp.scaleCooldown {
		return
	}

	// Calculate current load
	activeWorkers := atomic.LoadInt32(&wp.activeWorkers)
	totalWorkers := wp.TotalWorkers()

	if totalWorkers == 0 {
		return
	}

	utilization := float64(activeWorkers) / float64(totalWorkers)
	queueSize := len(wp.taskQueue)

	// Scale up if high utilization or large queue
	if (utilization > wp.scaleUpThreshold || queueSize > cap(wp.taskQueue)/2) && totalWorkers < wp.maxWorkers {
		if err := wp.addWorker(ctx); err == nil {
			wp.lastScaleTime = time.Now()
			wp.logger.Info("Scaled up worker pool",
				"utilization", utilization,
				"queueSize", queueSize,
				"totalWorkers", totalWorkers+1,
			)
		}
	} else if utilization < wp.scaleDownThreshold && queueSize == 0 && totalWorkers > wp.minWorkers {
		// Scale down if low utilization and empty queue
		if err := wp.removeWorker(); err == nil {
			wp.lastScaleTime = time.Now()
			wp.logger.Info("Scaled down worker pool",
				"utilization", utilization,
				"totalWorkers", totalWorkers-1,
			)
		}
	}
}

// reportMetrics reports worker pool metrics
func (wp *WorkerPool) reportMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics := wp.GetMetrics()

			// Publish metrics event
			event := events.NewEventBuilder("workerpool.metrics").
				WithPayload("metrics", metrics).
				Build()

			wp.eventBus.Publish(ctx, event)

			wp.logger.Info("Worker pool metrics",
				"totalWorkers", metrics.TotalWorkers,
				"activeWorkers", metrics.ActiveWorkers,
				"totalTasks", metrics.TotalTasks,
				"completedTasks", metrics.CompletedTasks,
				"failedTasks", metrics.FailedTasks,
				"queueSize", metrics.QueueSize,
			)

		case <-wp.stopCh:
			return
		}
	}
}

// GetMetrics returns current metrics for the worker pool
func (wp *WorkerPool) GetMetrics() WorkerPoolMetrics {
	return WorkerPoolMetrics{
		TotalWorkers:   wp.TotalWorkers(),
		ActiveWorkers:  wp.ActiveWorkers(),
		TotalTasks:     atomic.LoadInt64(&wp.totalTasks),
		CompletedTasks: atomic.LoadInt64(&wp.completedTasks),
		FailedTasks:    atomic.LoadInt64(&wp.failedTasks),
		QueueSize:      len(wp.taskQueue),
		QueueCapacity:  cap(wp.taskQueue),
	}
}

// WorkerPoolMetrics contains metrics for the worker pool
type WorkerPoolMetrics struct {
	TotalWorkers   int   `json:"totalWorkers"`
	ActiveWorkers  int   `json:"activeWorkers"`
	TotalTasks     int64 `json:"totalTasks"`
	CompletedTasks int64 `json:"completedTasks"`
	FailedTasks    int64 `json:"failedTasks"`
	QueueSize      int   `json:"queueSize"`
	QueueCapacity  int   `json:"queueCapacity"`
}
