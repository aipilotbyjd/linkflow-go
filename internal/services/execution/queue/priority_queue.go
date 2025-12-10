package queue

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// QueueItem represents an item in the priority queue
type QueueItem struct {
	Request     *workflow.ExecutionRequest `json:"request"`
	Priority    workflow.ExecutionPriority `json:"priority"`
	EnqueuedAt  time.Time                  `json:"enqueuedAt"`
	RetryCount  int                        `json:"retryCount"`
	LastRetryAt *time.Time                 `json:"lastRetryAt,omitempty"`
	index       int                        // Used by heap interface
}

// PriorityQueue implements a priority queue for execution requests
type PriorityQueue struct {
	mu       sync.RWMutex
	items    []*QueueItem
	priority workflow.ExecutionPriority
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(priority workflow.ExecutionPriority) *PriorityQueue {
	pq := &PriorityQueue{
		items:    make([]*QueueItem, 0),
		priority: priority,
	}
	heap.Init(pq)
	return pq
}

// Push adds an item to the queue
func (pq *PriorityQueue) Push(item *QueueItem) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	
	heap.Push(pq, item)
}

// Pop removes and returns the highest priority item
func (pq *PriorityQueue) Pop() *QueueItem {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	
	if len(pq.items) == 0 {
		return nil
	}
	
	item := heap.Pop(pq).(*QueueItem)
	return item
}

// Peek returns the highest priority item without removing it
func (pq *PriorityQueue) Peek() *QueueItem {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	
	if len(pq.items) == 0 {
		return nil
	}
	
	return pq.items[0]
}

// Size returns the number of items in the queue
func (pq *PriorityQueue) Size() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	
	return len(pq.items)
}

// IsEmpty checks if the queue is empty
func (pq *PriorityQueue) IsEmpty() bool {
	return pq.Size() == 0
}

// GetAll returns all items in the queue without removing them
func (pq *PriorityQueue) GetAll() []*QueueItem {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	
	items := make([]*QueueItem, len(pq.items))
	copy(items, pq.items)
	return items
}

// Remove removes a specific item from the queue
func (pq *PriorityQueue) Remove(id string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	
	for i, item := range pq.items {
		if item.Request.ID == id {
			heap.Remove(pq, i)
			return true
		}
	}
	
	return false
}

// Clear removes all items from the queue
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	
	pq.items = make([]*QueueItem, 0)
}

// Implement heap.Interface

func (pq *PriorityQueue) Len() int {
	return len(pq.items)
}

func (pq *PriorityQueue) Less(i, j int) bool {
	// Earlier enqueued items have higher priority (FIFO within same priority)
	return pq.items[i].EnqueuedAt.Before(pq.items[j].EnqueuedAt)
}

func (pq *PriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

// heap.Interface Push - DO NOT call directly, use Push method instead
func (pq *PriorityQueue) push(x interface{}) {
	n := len(pq.items)
	item := x.(*QueueItem)
	item.index = n
	pq.items = append(pq.items, item)
}

// heap.Interface Pop - DO NOT call directly, use Pop method instead  
func (pq *PriorityQueue) pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	pq.items = old[0 : n-1]
	return item
}

// For heap.Interface compatibility (lowercase methods used by heap package)
func (pq *PriorityQueue) Push(x interface{}) {
	pq.push(x)
}

func (pq *PriorityQueue) Pop() interface{} {
	return pq.pop()
}

// DeadLetterQueue handles failed execution requests
type DeadLetterQueue struct {
	mu         sync.RWMutex
	items      []*DeadLetterItem
	maxRetries int
	redis      *redis.Client
	logger     logger.Logger
}

// DeadLetterItem represents an item in the dead letter queue
type DeadLetterItem struct {
	Request       *workflow.ExecutionRequest `json:"request"`
	Error         string                     `json:"error"`
	FailedAt      time.Time                  `json:"failedAt"`
	RetryCount    int                        `json:"retryCount"`
	LastRetryAt   *time.Time                 `json:"lastRetryAt,omitempty"`
	CanRetry      bool                       `json:"canRetry"`
}

// NewDeadLetterQueue creates a new dead letter queue
func NewDeadLetterQueue(maxRetries int, redis *redis.Client, logger logger.Logger) *DeadLetterQueue {
	return &DeadLetterQueue{
		items:      make([]*DeadLetterItem, 0),
		maxRetries: maxRetries,
		redis:      redis,
		logger:     logger,
	}
}

// Add adds a failed request to the dead letter queue
func (dlq *DeadLetterQueue) Add(ctx context.Context, request *workflow.ExecutionRequest, err error) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	
	item := &DeadLetterItem{
		Request:    request,
		Error:      err.Error(),
		FailedAt:   time.Now(),
		RetryCount: 0,
		CanRetry:   true,
	}
	
	// Check if this is a retry
	for i, existing := range dlq.items {
		if existing.Request.ID == request.ID {
			existing.RetryCount++
			now := time.Now()
			existing.LastRetryAt = &now
			existing.Error = err.Error()
			existing.CanRetry = existing.RetryCount < dlq.maxRetries
			
			// Move to end of queue
			dlq.items = append(dlq.items[:i], dlq.items[i+1:]...)
			dlq.items = append(dlq.items, existing)
			
			dlq.persistItem(ctx, existing)
			return
		}
	}
	
	// New item
	dlq.items = append(dlq.items, item)
	dlq.persistItem(ctx, item)
	
	dlq.logger.Warn("Added to dead letter queue",
		"requestId", request.ID,
		"workflowId", request.WorkflowID,
		"error", err.Error(),
	)
}

// GetRetryable returns all items that can be retried
func (dlq *DeadLetterQueue) GetRetryable() []*DeadLetterItem {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	
	var retryable []*DeadLetterItem
	for _, item := range dlq.items {
		if item.CanRetry {
			retryable = append(retryable, item)
		}
	}
	
	return retryable
}

// Remove removes an item from the dead letter queue
func (dlq *DeadLetterQueue) Remove(requestID string) bool {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	
	for i, item := range dlq.items {
		if item.Request.ID == requestID {
			dlq.items = append(dlq.items[:i], dlq.items[i+1:]...)
			dlq.removePersistedItem(context.Background(), requestID)
			return true
		}
	}
	
	return false
}

// Size returns the number of items in the dead letter queue
func (dlq *DeadLetterQueue) Size() int {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	
	return len(dlq.items)
}

// GetAll returns all items in the dead letter queue
func (dlq *DeadLetterQueue) GetAll() []*DeadLetterItem {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	
	items := make([]*DeadLetterItem, len(dlq.items))
	copy(items, dlq.items)
	return items
}

// persistItem persists a dead letter item to Redis
func (dlq *DeadLetterQueue) persistItem(ctx context.Context, item *DeadLetterItem) {
	key := fmt.Sprintf("dlq:item:%s", item.Request.ID)
	data, err := json.Marshal(item)
	if err != nil {
		dlq.logger.Error("Failed to marshal dead letter item", "error", err)
		return
	}
	
	// Persist with TTL of 7 days
	if err := dlq.redis.Set(ctx, key, data, 7*24*time.Hour).Err(); err != nil {
		dlq.logger.Error("Failed to persist dead letter item", "error", err)
	}
}

// removePersistedItem removes a persisted dead letter item
func (dlq *DeadLetterQueue) removePersistedItem(ctx context.Context, requestID string) {
	key := fmt.Sprintf("dlq:item:%s", requestID)
	if err := dlq.redis.Del(ctx, key).Err(); err != nil {
		dlq.logger.Error("Failed to remove persisted dead letter item", "error", err)
	}
}

// RestoreFromRedis restores dead letter items from Redis
func (dlq *DeadLetterQueue) RestoreFromRedis(ctx context.Context) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	
	// Scan for all dead letter items
	iter := dlq.redis.Scan(ctx, 0, "dlq:item:*", 0).Iterator()
	restored := 0
	
	for iter.Next(ctx) {
		key := iter.Val()
		data, err := dlq.redis.Get(ctx, key).Result()
		if err != nil {
			dlq.logger.Error("Failed to get dead letter item", "key", key, "error", err)
			continue
		}
		
		var item DeadLetterItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			dlq.logger.Error("Failed to unmarshal dead letter item", "key", key, "error", err)
			continue
		}
		
		dlq.items = append(dlq.items, &item)
		restored++
	}
	
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan dead letter items: %w", err)
	}
	
	dlq.logger.Info("Restored dead letter items from Redis", "count", restored)
	return nil
}
