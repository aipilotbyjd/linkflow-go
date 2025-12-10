package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// QueueManager manages execution queues with different priorities
type QueueManager struct {
	mu            sync.RWMutex
	highQueue     *PriorityQueue
	normalQueue   *PriorityQueue
	lowQueue      *PriorityQueue
	workerPool    *WorkerPool
	redis         *redis.Client
	eventBus      events.EventBus
	logger        logger.Logger
	
	// Metrics
	queuedCount   int64
	processingCount int64
	completedCount int64
	failedCount    int64
	
	// Configuration
	maxQueueSize  int
	persistQueue  bool
	deadLetterQueue *DeadLetterQueue
	
	// Control
	stopCh        chan struct{}
	stopped       atomic.Bool
}

// QueueConfig contains configuration for the queue manager
type QueueConfig struct {
	MaxQueueSize     int
	PersistToRedis   bool
	EnableDeadLetter bool
	MaxRetries       int
	WorkerCount      int
}

// NewQueueManager creates a new queue manager
func NewQueueManager(
	config QueueConfig,
	workerPool *WorkerPool,
	redis *redis.Client,
	eventBus events.EventBus,
	logger logger.Logger,
) *QueueManager {
	qm := &QueueManager{
		highQueue:    NewPriorityQueue(workflow.PriorityHigh),
		normalQueue:  NewPriorityQueue(workflow.PriorityNormal),
		lowQueue:     NewPriorityQueue(workflow.PriorityLow),
		workerPool:   workerPool,
		redis:        redis,
		eventBus:     eventBus,
		logger:       logger,
		maxQueueSize: config.MaxQueueSize,
		persistQueue: config.PersistToRedis,
		stopCh:       make(chan struct{}),
	}
	
	if config.EnableDeadLetter {
		qm.deadLetterQueue = NewDeadLetterQueue(config.MaxRetries, redis, logger)
	}
	
	return qm
}

// Start starts the queue manager
func (qm *QueueManager) Start(ctx context.Context) error {
	qm.logger.Info("Starting queue manager")
	
	// Restore persisted queues if enabled
	if qm.persistQueue {
		if err := qm.restoreQueues(ctx); err != nil {
			qm.logger.Error("Failed to restore queues", "error", err)
		}
	}
	
	// Start queue processors
	go qm.processQueues(ctx)
	
	// Start metrics reporter
	go qm.reportMetrics(ctx)
	
	// Start queue persistence
	if qm.persistQueue {
		go qm.persistQueues(ctx)
	}
	
	return nil
}

// Stop stops the queue manager
func (qm *QueueManager) Stop(ctx context.Context) error {
	qm.logger.Info("Stopping queue manager")
	qm.stopped.Store(true)
	close(qm.stopCh)
	
	// Persist remaining queue items
	if qm.persistQueue {
		if err := qm.saveQueues(ctx); err != nil {
			qm.logger.Error("Failed to save queues on shutdown", "error", err)
		}
	}
	
	return nil
}

// Enqueue adds an execution request to the appropriate queue
func (qm *QueueManager) Enqueue(ctx context.Context, request *workflow.ExecutionRequest) error {
	if qm.stopped.Load() {
		return fmt.Errorf("queue manager is stopped")
	}
	
	// Check queue size limit
	if qm.getQueueSize() >= qm.maxQueueSize {
		return fmt.Errorf("queue is full (max size: %d)", qm.maxQueueSize)
	}
	
	// Add to appropriate queue based on priority
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	var queue *PriorityQueue
	switch request.Priority {
	case workflow.PriorityHigh:
		queue = qm.highQueue
	case workflow.PriorityLow:
		queue = qm.lowQueue
	default:
		queue = qm.normalQueue
	}
	
	// Create queue item
	item := &QueueItem{
		Request:   request,
		Priority:  request.Priority,
		EnqueuedAt: time.Now(),
	}
	
	// Add to queue
	queue.Push(item)
	atomic.AddInt64(&qm.queuedCount, 1)
	
	// Persist to Redis if enabled
	if qm.persistQueue {
		go qm.persistItem(ctx, item)
	}
	
	// Publish enqueued event
	event := events.NewEventBuilder(events.ExecutionQueued).
		WithAggregateID(request.ID).
		WithAggregateType("execution").
		WithPayload("workflowId", request.WorkflowID).
		WithPayload("priority", string(request.Priority)).
		Build()
	
	if err := qm.eventBus.Publish(ctx, event); err != nil {
		qm.logger.Error("Failed to publish enqueued event", "error", err)
	}
	
	qm.logger.Info("Execution request enqueued",
		"requestId", request.ID,
		"workflowId", request.WorkflowID,
		"priority", request.Priority,
	)
	
	return nil
}

// Dequeue removes and returns the next execution request
func (qm *QueueManager) Dequeue(ctx context.Context) (*workflow.ExecutionRequest, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	// Try high priority first, then normal, then low
	var item *QueueItem
	
	if !qm.highQueue.IsEmpty() {
		item = qm.highQueue.Pop()
	} else if !qm.normalQueue.IsEmpty() {
		item = qm.normalQueue.Pop()
	} else if !qm.lowQueue.IsEmpty() {
		item = qm.lowQueue.Pop()
	}
	
	if item == nil {
		return nil, fmt.Errorf("no items in queue")
	}
	
	atomic.AddInt64(&qm.processingCount, 1)
	atomic.AddInt64(&qm.queuedCount, -1)
	
	// Remove from persistence if enabled
	if qm.persistQueue {
		go qm.removePersistedItem(ctx, item.Request.ID)
	}
	
	return item.Request, nil
}

// processQueues continuously processes items from queues
func (qm *QueueManager) processQueues(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-qm.stopCh:
			return
		case <-ticker.C:
			qm.processNextItem(ctx)
		}
	}
}

// processNextItem processes the next item in the queue
func (qm *QueueManager) processNextItem(ctx context.Context) {
	// Check if workers are available
	if !qm.workerPool.HasAvailableWorker() {
		return
	}
	
	// Dequeue next item
	request, err := qm.Dequeue(ctx)
	if err != nil {
		// No items to process
		return
	}
	
	// Submit to worker pool
	task := &WorkerTask{
		ID:         request.ID,
		WorkflowID: request.WorkflowID,
		Request:    request,
		CreatedAt:  time.Now(),
	}
	
	if err := qm.workerPool.Submit(ctx, task); err != nil {
		qm.logger.Error("Failed to submit task to worker pool", 
			"error", err,
			"taskId", task.ID,
		)
		
		// Re-queue the request
		qm.Enqueue(ctx, request)
	}
}

// GetQueueStatus returns the current queue status
func (qm *QueueManager) GetQueueStatus() QueueStatus {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	return QueueStatus{
		HighPriority:    qm.highQueue.Size(),
		NormalPriority:  qm.normalQueue.Size(),
		LowPriority:     qm.lowQueue.Size(),
		TotalQueued:     atomic.LoadInt64(&qm.queuedCount),
		Processing:      atomic.LoadInt64(&qm.processingCount),
		Completed:       atomic.LoadInt64(&qm.completedCount),
		Failed:          atomic.LoadInt64(&qm.failedCount),
		WorkersActive:   qm.workerPool.ActiveWorkers(),
		WorkersTotal:    qm.workerPool.TotalWorkers(),
	}
}

// getQueueSize returns the total size of all queues
func (qm *QueueManager) getQueueSize() int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	return qm.highQueue.Size() + qm.normalQueue.Size() + qm.lowQueue.Size()
}

// HandleExecutionComplete handles execution completion
func (qm *QueueManager) HandleExecutionComplete(ctx context.Context, executionID string, success bool) {
	atomic.AddInt64(&qm.processingCount, -1)
	
	if success {
		atomic.AddInt64(&qm.completedCount, 1)
	} else {
		atomic.AddInt64(&qm.failedCount, 1)
		
		// Check if should be sent to dead letter queue
		if qm.deadLetterQueue != nil {
			// Retrieve original request from tracking
			if request, err := qm.retrieveRequest(ctx, executionID); err == nil {
				qm.deadLetterQueue.Add(ctx, request, fmt.Errorf("execution failed"))
			}
		}
	}
}

// Persistence methods

func (qm *QueueManager) persistQueues(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-qm.stopCh:
			return
		case <-ticker.C:
			if err := qm.saveQueues(ctx); err != nil {
				qm.logger.Error("Failed to persist queues", "error", err)
			}
		}
	}
}

func (qm *QueueManager) saveQueues(ctx context.Context) error {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	// Save each queue
	queues := map[string]*PriorityQueue{
		"queue:high":   qm.highQueue,
		"queue:normal": qm.normalQueue,
		"queue:low":    qm.lowQueue,
	}
	
	for key, queue := range queues {
		items := queue.GetAll()
		if len(items) == 0 {
			continue
		}
		
		// Serialize items
		data, err := json.Marshal(items)
		if err != nil {
			return fmt.Errorf("failed to marshal queue %s: %w", key, err)
		}
		
		// Save to Redis
		if err := qm.redis.Set(ctx, key, data, 0).Err(); err != nil {
			return fmt.Errorf("failed to save queue %s: %w", key, err)
		}
	}
	
	return nil
}

func (qm *QueueManager) restoreQueues(ctx context.Context) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	// Restore each queue
	queues := map[string]*PriorityQueue{
		"queue:high":   qm.highQueue,
		"queue:normal": qm.normalQueue,
		"queue:low":    qm.lowQueue,
	}
	
	for key, queue := range queues {
		// Get from Redis
		data, err := qm.redis.Get(ctx, key).Result()
		if err == redis.Nil {
			continue // No data
		} else if err != nil {
			return fmt.Errorf("failed to get queue %s: %w", key, err)
		}
		
		// Deserialize items
		var items []*QueueItem
		if err := json.Unmarshal([]byte(data), &items); err != nil {
			return fmt.Errorf("failed to unmarshal queue %s: %w", key, err)
		}
		
		// Restore to queue
		for _, item := range items {
			queue.Push(item)
		}
		
		qm.logger.Info("Restored queue", "queue", key, "items", len(items))
	}
	
	return nil
}

func (qm *QueueManager) persistItem(ctx context.Context, item *QueueItem) {
	key := fmt.Sprintf("queue:item:%s", item.Request.ID)
	data, err := json.Marshal(item)
	if err != nil {
		qm.logger.Error("Failed to marshal queue item", "error", err, "id", item.Request.ID)
		return
	}
	
	if err := qm.redis.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
		qm.logger.Error("Failed to persist queue item", "error", err, "id", item.Request.ID)
	}
}

func (qm *QueueManager) removePersistedItem(ctx context.Context, id string) {
	key := fmt.Sprintf("queue:item:%s", id)
	if err := qm.redis.Del(ctx, key).Err(); err != nil {
		qm.logger.Error("Failed to remove persisted item", "error", err, "id", id)
	}
}

func (qm *QueueManager) retrieveRequest(ctx context.Context, executionID string) (*workflow.ExecutionRequest, error) {
	key := fmt.Sprintf("queue:item:%s", executionID)
	data, err := qm.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	
	var item QueueItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, err
	}
	
	return item.Request, nil
}

// reportMetrics reports queue metrics
func (qm *QueueManager) reportMetrics(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-qm.stopCh:
			return
		case <-ticker.C:
			status := qm.GetQueueStatus()
			
			// Publish metrics event
			event := events.NewEventBuilder("queue.metrics").
				WithPayload("status", status).
				Build()
			
			qm.eventBus.Publish(ctx, event)
			
			qm.logger.Info("Queue metrics",
				"queued", status.TotalQueued,
				"processing", status.Processing,
				"completed", status.Completed,
				"failed", status.Failed,
			)
		}
	}
}

// QueueStatus represents the current status of the queues
type QueueStatus struct {
	HighPriority   int   `json:"highPriority"`
	NormalPriority int   `json:"normalPriority"`
	LowPriority    int   `json:"lowPriority"`
	TotalQueued    int64 `json:"totalQueued"`
	Processing     int64 `json:"processing"`
	Completed      int64 `json:"completed"`
	Failed         int64 `json:"failed"`
	WorkersActive  int   `json:"workersActive"`
	WorkersTotal   int   `json:"workersTotal"`
}
