package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// WorkerRegistry manages worker registration and discovery
type WorkerRegistry struct {
	mu       sync.RWMutex
	backend  RegistryBackend
	cache    map[string]*WorkerNode
	logger   logger.Logger
	
	// Configuration
	ttl             time.Duration
	refreshInterval time.Duration
	
	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// RegistryBackend interface for different registry implementations
type RegistryBackend interface {
	Register(ctx context.Context, worker *WorkerNode, ttl time.Duration) error
	Unregister(ctx context.Context, workerID string) error
	Get(ctx context.Context, workerID string) (*WorkerNode, error)
	List(ctx context.Context) ([]*WorkerNode, error)
	Refresh(ctx context.Context, workerID string, ttl time.Duration) error
	Watch(ctx context.Context) (<-chan RegistryEvent, error)
}

// RegistryEvent represents an event from the registry
type RegistryEvent struct {
	Type     EventType
	WorkerID string
	Worker   *WorkerNode
}

// EventType represents the type of registry event
type EventType string

const (
	EventTypeWorkerAdded   EventType = "worker_added"
	EventTypeWorkerUpdated EventType = "worker_updated"
	EventTypeWorkerRemoved EventType = "worker_removed"
)

// NewWorkerRegistry creates a new worker registry
func NewWorkerRegistry(backend RegistryBackend, logger logger.Logger) *WorkerRegistry {
	return &WorkerRegistry{
		backend:         backend,
		cache:           make(map[string]*WorkerNode),
		logger:          logger,
		ttl:             30 * time.Second,
		refreshInterval: 10 * time.Second,
		stopCh:          make(chan struct{}),
	}
}

// Start starts the registry
func (r *WorkerRegistry) Start(ctx context.Context) error {
	r.logger.Info("Starting worker registry")
	
	// Load existing workers
	if err := r.refresh(ctx); err != nil {
		r.logger.Error("Failed to load workers", "error", err)
	}
	
	// Start watching for changes
	r.wg.Add(2)
	go r.watchLoop(ctx)
	go r.refreshLoop(ctx)
	
	return nil
}

// Stop stops the registry
func (r *WorkerRegistry) Stop(ctx context.Context) error {
	r.logger.Info("Stopping worker registry")
	
	close(r.stopCh)
	
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		r.logger.Info("Registry stopped")
	case <-ctx.Done():
		r.logger.Warn("Registry stop timeout")
	}
	
	return nil
}

// Register registers a worker
func (r *WorkerRegistry) Register(ctx context.Context, worker *WorkerNode) error {
	if err := r.backend.Register(ctx, worker, r.ttl); err != nil {
		return fmt.Errorf("failed to register worker: %w", err)
	}
	
	r.mu.Lock()
	r.cache[worker.ID] = worker
	r.mu.Unlock()
	
	r.logger.Info("Worker registered", "workerId", worker.ID)
	return nil
}

// Unregister unregisters a worker
func (r *WorkerRegistry) Unregister(ctx context.Context, workerID string) error {
	if err := r.backend.Unregister(ctx, workerID); err != nil {
		return fmt.Errorf("failed to unregister worker: %w", err)
	}
	
	r.mu.Lock()
	delete(r.cache, workerID)
	r.mu.Unlock()
	
	r.logger.Info("Worker unregistered", "workerId", workerID)
	return nil
}

// GetWorker gets a specific worker
func (r *WorkerRegistry) GetWorker(ctx context.Context, workerID string) (*WorkerNode, error) {
	// Check cache first
	r.mu.RLock()
	if worker, exists := r.cache[workerID]; exists {
		r.mu.RUnlock()
		return worker, nil
	}
	r.mu.RUnlock()
	
	// Fetch from backend
	worker, err := r.backend.Get(ctx, workerID)
	if err != nil {
		return nil, err
	}
	
	// Update cache
	r.mu.Lock()
	r.cache[workerID] = worker
	r.mu.Unlock()
	
	return worker, nil
}

// ListWorkers lists all workers
func (r *WorkerRegistry) ListWorkers(ctx context.Context) ([]*WorkerNode, error) {
	workers, err := r.backend.List(ctx)
	if err != nil {
		return nil, err
	}
	
	// Update cache
	r.mu.Lock()
	for _, worker := range workers {
		r.cache[worker.ID] = worker
	}
	r.mu.Unlock()
	
	return workers, nil
}

// RefreshWorker refreshes a worker's registration
func (r *WorkerRegistry) RefreshWorker(ctx context.Context, workerID string) error {
	return r.backend.Refresh(ctx, workerID, r.ttl)
}

// watchLoop watches for registry events
func (r *WorkerRegistry) watchLoop(ctx context.Context) {
	defer r.wg.Done()
	
	eventCh, err := r.backend.Watch(ctx)
	if err != nil {
		r.logger.Error("Failed to start watch", "error", err)
		return
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case event, ok := <-eventCh:
			if !ok {
				r.logger.Warn("Watch channel closed")
				return
			}
			
			r.handleRegistryEvent(event)
		}
	}
}

// handleRegistryEvent handles a registry event
func (r *WorkerRegistry) handleRegistryEvent(event RegistryEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	switch event.Type {
	case EventTypeWorkerAdded, EventTypeWorkerUpdated:
		r.cache[event.WorkerID] = event.Worker
		r.logger.Debug("Worker cache updated", "workerId", event.WorkerID, "event", event.Type)
		
	case EventTypeWorkerRemoved:
		delete(r.cache, event.WorkerID)
		r.logger.Debug("Worker removed from cache", "workerId", event.WorkerID)
	}
}

// refreshLoop periodically refreshes the cache
func (r *WorkerRegistry) refreshLoop(ctx context.Context) {
	defer r.wg.Done()
	
	ticker := time.NewTicker(r.refreshInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.refresh(ctx); err != nil {
				r.logger.Error("Failed to refresh workers", "error", err)
			}
		}
	}
}

// refresh refreshes the worker cache
func (r *WorkerRegistry) refresh(ctx context.Context) error {
	workers, err := r.backend.List(ctx)
	if err != nil {
		return err
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Clear and rebuild cache
	r.cache = make(map[string]*WorkerNode)
	for _, worker := range workers {
		r.cache[worker.ID] = worker
	}
	
	r.logger.Debug("Worker cache refreshed", "workers", len(workers))
	return nil
}

// RedisBackend implements RegistryBackend using Redis
type RedisBackend struct {
	client *redis.Client
	prefix string
	logger logger.Logger
}

// NewRedisBackend creates a new Redis backend
func NewRedisBackend(client *redis.Client, prefix string, logger logger.Logger) *RedisBackend {
	if prefix == "" {
		prefix = "worker:registry:"
	}
	
	return &RedisBackend{
		client: client,
		prefix: prefix,
		logger: logger,
	}
}

// Register registers a worker in Redis
func (rb *RedisBackend) Register(ctx context.Context, worker *WorkerNode, ttl time.Duration) error {
	key := rb.prefix + worker.ID
	
	data, err := json.Marshal(worker)
	if err != nil {
		return fmt.Errorf("failed to marshal worker: %w", err)
	}
	
	if err := rb.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set worker in Redis: %w", err)
	}
	
	// Add to worker set
	if err := rb.client.SAdd(ctx, rb.prefix+"workers", worker.ID).Err(); err != nil {
		return fmt.Errorf("failed to add worker to set: %w", err)
	}
	
	return nil
}

// Unregister removes a worker from Redis
func (rb *RedisBackend) Unregister(ctx context.Context, workerID string) error {
	key := rb.prefix + workerID
	
	if err := rb.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete worker: %w", err)
	}
	
	// Remove from worker set
	if err := rb.client.SRem(ctx, rb.prefix+"workers", workerID).Err(); err != nil {
		return fmt.Errorf("failed to remove worker from set: %w", err)
	}
	
	return nil
}

// Get gets a worker from Redis
func (rb *RedisBackend) Get(ctx context.Context, workerID string) (*WorkerNode, error) {
	key := rb.prefix + workerID
	
	data, err := rb.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("worker not found: %s", workerID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get worker: %w", err)
	}
	
	var worker WorkerNode
	if err := json.Unmarshal([]byte(data), &worker); err != nil {
		return nil, fmt.Errorf("failed to unmarshal worker: %w", err)
	}
	
	return &worker, nil
}

// List lists all workers from Redis
func (rb *RedisBackend) List(ctx context.Context) ([]*WorkerNode, error) {
	// Get all worker IDs
	workerIDs, err := rb.client.SMembers(ctx, rb.prefix+"workers").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get worker IDs: %w", err)
	}
	
	workers := make([]*WorkerNode, 0, len(workerIDs))
	
	for _, id := range workerIDs {
		worker, err := rb.Get(ctx, id)
		if err != nil {
			rb.logger.Warn("Failed to get worker", "workerId", id, "error", err)
			// Remove from set if not found
			rb.client.SRem(ctx, rb.prefix+"workers", id)
			continue
		}
		workers = append(workers, worker)
	}
	
	return workers, nil
}

// Refresh refreshes a worker's TTL in Redis
func (rb *RedisBackend) Refresh(ctx context.Context, workerID string, ttl time.Duration) error {
	key := rb.prefix + workerID
	
	// Check if exists
	exists, err := rb.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}
	
	if exists == 0 {
		return fmt.Errorf("worker not found: %s", workerID)
	}
	
	// Update TTL
	if err := rb.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to update TTL: %w", err)
	}
	
	return nil
}

// Watch watches for changes (simplified for Redis)
func (rb *RedisBackend) Watch(ctx context.Context) (<-chan RegistryEvent, error) {
	// Redis doesn't have built-in watch like etcd
	// This is a simplified implementation using pub/sub
	eventCh := make(chan RegistryEvent, 100)
	
	// Subscribe to worker events
	pubsub := rb.client.Subscribe(ctx, rb.prefix+"events")
	
	go func() {
		defer close(eventCh)
		defer pubsub.Close()
		
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-pubsub.Channel():
				var event RegistryEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					rb.logger.Error("Failed to unmarshal event", "error", err)
					continue
				}
				
				select {
				case eventCh <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	
	return eventCh, nil
}

// EtcdBackend implements RegistryBackend using etcd
type EtcdBackend struct {
	client *clientv3.Client
	prefix string
	logger logger.Logger
}

// NewEtcdBackend creates a new etcd backend
func NewEtcdBackend(endpoints []string, prefix string, logger logger.Logger) (*EtcdBackend, error) {
	if prefix == "" {
		prefix = "/workers/"
	}
	
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}
	
	return &EtcdBackend{
		client: client,
		prefix: prefix,
		logger: logger,
	}, nil
}

// Register registers a worker in etcd
func (eb *EtcdBackend) Register(ctx context.Context, worker *WorkerNode, ttl time.Duration) error {
	key := eb.prefix + worker.ID
	
	data, err := json.Marshal(worker)
	if err != nil {
		return fmt.Errorf("failed to marshal worker: %w", err)
	}
	
	// Create lease
	lease, err := eb.client.Grant(ctx, int64(ttl.Seconds()))
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}
	
	// Put with lease
	_, err = eb.client.Put(ctx, key, string(data), clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("failed to register worker: %w", err)
	}
	
	return nil
}

// Unregister removes a worker from etcd
func (eb *EtcdBackend) Unregister(ctx context.Context, workerID string) error {
	key := eb.prefix + workerID
	
	_, err := eb.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to unregister worker: %w", err)
	}
	
	return nil
}

// Get gets a worker from etcd
func (eb *EtcdBackend) Get(ctx context.Context, workerID string) (*WorkerNode, error) {
	key := eb.prefix + workerID
	
	resp, err := eb.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker: %w", err)
	}
	
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("worker not found: %s", workerID)
	}
	
	var worker WorkerNode
	if err := json.Unmarshal(resp.Kvs[0].Value, &worker); err != nil {
		return nil, fmt.Errorf("failed to unmarshal worker: %w", err)
	}
	
	return &worker, nil
}

// List lists all workers from etcd
func (eb *EtcdBackend) List(ctx context.Context) ([]*WorkerNode, error) {
	resp, err := eb.client.Get(ctx, eb.prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}
	
	workers := make([]*WorkerNode, 0, len(resp.Kvs))
	
	for _, kv := range resp.Kvs {
		var worker WorkerNode
		if err := json.Unmarshal(kv.Value, &worker); err != nil {
			eb.logger.Warn("Failed to unmarshal worker", "key", string(kv.Key), "error", err)
			continue
		}
		workers = append(workers, &worker)
	}
	
	return workers, nil
}

// Refresh refreshes a worker's lease in etcd
func (eb *EtcdBackend) Refresh(ctx context.Context, workerID string, ttl time.Duration) error {
	// In etcd, we need to keep-alive the lease
	// This is simplified - in production, would maintain lease IDs
	worker, err := eb.Get(ctx, workerID)
	if err != nil {
		return err
	}
	
	return eb.Register(ctx, worker, ttl)
}

// Watch watches for changes in etcd
func (eb *EtcdBackend) Watch(ctx context.Context) (<-chan RegistryEvent, error) {
	eventCh := make(chan RegistryEvent, 100)
	
	// Watch prefix
	watchCh := eb.client.Watch(ctx, eb.prefix, clientv3.WithPrefix())
	
	go func() {
		defer close(eventCh)
		
		for watchResp := range watchCh {
			for _, event := range watchResp.Events {
				var registryEvent RegistryEvent
				
				// Extract worker ID from key
				workerID := string(event.Kv.Key)[len(eb.prefix):]
				registryEvent.WorkerID = workerID
				
				switch event.Type {
				case clientv3.EventTypePut:
					var worker WorkerNode
					if err := json.Unmarshal(event.Kv.Value, &worker); err != nil {
						eb.logger.Error("Failed to unmarshal worker", "error", err)
						continue
					}
					
					if event.IsCreate() {
						registryEvent.Type = EventTypeWorkerAdded
					} else {
						registryEvent.Type = EventTypeWorkerUpdated
					}
					registryEvent.Worker = &worker
					
				case clientv3.EventTypeDelete:
					registryEvent.Type = EventTypeWorkerRemoved
				}
				
				select {
				case eventCh <- registryEvent:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	
	return eventCh, nil
}
