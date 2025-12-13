package distributed

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// Coordinator manages distributed execution across multiple workers
type Coordinator struct {
	mu              sync.RWMutex
	workers         map[string]*WorkerNode
	partitions      map[string]string // executionID -> workerID mapping
	workDistributor *WorkDistributor
	registry        *WorkerRegistry
	redis           *redis.Client
	eventBus        events.EventBus
	logger          logger.Logger
	
	// Configuration
	rebalanceInterval   time.Duration
	healthCheckInterval time.Duration
	maxWorkPerWorker    int
	
	// Metrics
	totalExecutions     int64
	distributedWork     int64
	failedDistributions int64
	
	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// WorkerNode represents a worker node in the distributed system
type WorkerNode struct {
	ID           string            `json:"id"`
	Address      string            `json:"address"`
	Capacity     int               `json:"capacity"`
	CurrentLoad  int               `json:"currentLoad"`
	Tags         []string          `json:"tags"`
	Capabilities []string          `json:"capabilities"`
	Status       WorkerStatus      `json:"status"`
	LastHeartbeat time.Time        `json:"lastHeartbeat"`
	RegisteredAt time.Time         `json:"registeredAt"`
	Metadata     map[string]string `json:"metadata"`
	
	// Performance metrics
	ExecutionsCompleted int64         `json:"executionsCompleted"`
	ExecutionsFailed    int64         `json:"executionsFailed"`
	AverageExecutionTime time.Duration `json:"averageExecutionTime"`
	
	mu sync.RWMutex
}

// WorkerStatus represents the status of a worker
type WorkerStatus string

const (
	WorkerStatusActive      WorkerStatus = "active"
	WorkerStatusUnhealthy   WorkerStatus = "unhealthy"
	WorkerStatusDraining    WorkerStatus = "draining"
	WorkerStatusOffline     WorkerStatus = "offline"
)

// CoordinatorConfig contains configuration for the coordinator
type CoordinatorConfig struct {
	RebalanceInterval   time.Duration
	HealthCheckInterval time.Duration
	MaxWorkPerWorker    int
}

// NewCoordinator creates a new distributed coordinator
func NewCoordinator(
	config CoordinatorConfig,
	registry *WorkerRegistry,
	redis *redis.Client,
	eventBus events.EventBus,
	logger logger.Logger,
) *Coordinator {
	if config.RebalanceInterval == 0 {
		config.RebalanceInterval = 30 * time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 10 * time.Second
	}
	if config.MaxWorkPerWorker == 0 {
		config.MaxWorkPerWorker = 100
	}
	
	coord := &Coordinator{
		workers:             make(map[string]*WorkerNode),
		partitions:          make(map[string]string),
		registry:            registry,
		redis:               redis,
		eventBus:            eventBus,
		logger:              logger,
		rebalanceInterval:   config.RebalanceInterval,
		healthCheckInterval: config.HealthCheckInterval,
		maxWorkPerWorker:    config.MaxWorkPerWorker,
		stopCh:              make(chan struct{}),
	}
	
	coord.workDistributor = NewWorkDistributor(coord, logger)
	
	return coord
}

// Start starts the coordinator
func (c *Coordinator) Start(ctx context.Context) error {
	c.logger.Info("Starting distributed coordinator")
	
	// Subscribe to worker events
	if err := c.subscribeToEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}
	
	// Load existing workers from registry
	if err := c.loadWorkers(ctx); err != nil {
		c.logger.Error("Failed to load workers from registry", "error", err)
	}
	
	// Start background tasks
	c.wg.Add(3)
	go c.healthCheckLoop(ctx)
	go c.rebalanceLoop(ctx)
	go c.metricsLoop(ctx)
	
	return nil
}

// Stop stops the coordinator
func (c *Coordinator) Stop(ctx context.Context) error {
	c.logger.Info("Stopping distributed coordinator")
	
	close(c.stopCh)
	
	// Wait for background tasks
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		c.logger.Info("Coordinator stopped gracefully")
	case <-ctx.Done():
		c.logger.Warn("Coordinator stop timeout")
	}
	
	return nil
}

// AssignWork assigns work to an appropriate worker
func (c *Coordinator) AssignWork(ctx context.Context, executionID string, workflowID string, requirements WorkRequirements) (*WorkerNode, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if already assigned
	if workerID, exists := c.partitions[executionID]; exists {
		if worker, ok := c.workers[workerID]; ok && worker.Status == WorkerStatusActive {
			return worker, nil
		}
		// Worker no longer available, reassign
		delete(c.partitions, executionID)
	}
	
	// Find suitable worker
	worker := c.selectWorker(requirements)
	if worker == nil {
		return nil, fmt.Errorf("no suitable worker available")
	}
	
	// Assign work
	c.partitions[executionID] = worker.ID
	worker.CurrentLoad++
	
	atomic.AddInt64(&c.distributedWork, 1)
	
	// Publish assignment event
	event := events.NewEventBuilder("work.assigned").
		WithAggregateID(executionID).
		WithPayload("workerId", worker.ID).
		WithPayload("workflowId", workflowID).
		Build()
	
	c.eventBus.Publish(ctx, event)
	
	c.logger.Info("Work assigned",
		"executionId", executionID,
		"workerId", worker.ID,
		"workerLoad", worker.CurrentLoad,
	)
	
	return worker, nil
}

// selectWorker selects the best worker based on requirements and load
func (c *Coordinator) selectWorker(requirements WorkRequirements) *WorkerNode {
	var candidates []*WorkerNode
	
	// Filter eligible workers
	for _, worker := range c.workers {
		if worker.Status != WorkerStatusActive {
			continue
		}
		
		if worker.CurrentLoad >= worker.Capacity {
			continue
		}
		
		// Check requirements
		if requirements.RequiresTags != nil {
			hasAllTags := true
			for _, reqTag := range requirements.RequiresTags {
				found := false
				for _, tag := range worker.Tags {
					if tag == reqTag {
						found = true
						break
					}
				}
				if !found {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}
		
		candidates = append(candidates, worker)
	}
	
	if len(candidates) == 0 {
		return nil
	}
	
	// Select based on strategy
	switch requirements.SelectionStrategy {
	case SelectionStrategyLeastLoaded:
		return c.selectLeastLoaded(candidates)
	case SelectionStrategyRoundRobin:
		return c.selectRoundRobin(candidates)
	case SelectionStrategyRandom:
		return c.selectRandom(candidates)
	case SelectionStrategyAffinity:
		return c.selectWithAffinity(candidates, requirements.AffinityKey)
	default:
		return c.selectLeastLoaded(candidates)
	}
}

// selectLeastLoaded selects the worker with the lowest load
func (c *Coordinator) selectLeastLoaded(candidates []*WorkerNode) *WorkerNode {
	var selected *WorkerNode
	minLoad := int(^uint(0) >> 1) // Max int
	
	for _, worker := range candidates {
		loadPercentage := float64(worker.CurrentLoad) / float64(worker.Capacity)
		currentLoad := int(loadPercentage * 100)
		
		if currentLoad < minLoad {
			minLoad = currentLoad
			selected = worker
		}
	}
	
	return selected
}

// selectRoundRobin selects workers in round-robin fashion
func (c *Coordinator) selectRoundRobin(candidates []*WorkerNode) *WorkerNode {
	// Simple round-robin: select next worker in list
	if len(candidates) == 0 {
		return nil
	}
	
	// Use distributed work count as index
	index := atomic.LoadInt64(&c.distributedWork) % int64(len(candidates))
	return candidates[index]
}

// selectRandom selects a random worker
func (c *Coordinator) selectRandom(candidates []*WorkerNode) *WorkerNode {
	if len(candidates) == 0 {
		return nil
	}
	
	index := rand.Intn(len(candidates))
	return candidates[index]
}

// selectWithAffinity selects a worker based on affinity key
func (c *Coordinator) selectWithAffinity(candidates []*WorkerNode, affinityKey string) *WorkerNode {
	if affinityKey == "" || len(candidates) == 0 {
		return c.selectLeastLoaded(candidates)
	}
	
	// Hash affinity key to select worker
	hash := 0
	for _, ch := range affinityKey {
		hash = hash*31 + int(ch)
	}
	
	index := hash % len(candidates)
	if index < 0 {
		index = -index
	}
	
	return candidates[index]
}

// RegisterWorker registers a new worker
func (c *Coordinator) RegisterWorker(ctx context.Context, worker *WorkerNode) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Validate worker
	if worker.ID == "" {
		return fmt.Errorf("worker ID is required")
	}
	
	// Set defaults
	if worker.Capacity == 0 {
		worker.Capacity = 10
	}
	
	worker.Status = WorkerStatusActive
	worker.LastHeartbeat = time.Now()
	worker.RegisteredAt = time.Now()
	
	// Store worker
	c.workers[worker.ID] = worker
	
	// Register with registry
	if err := c.registry.Register(ctx, worker); err != nil {
		return fmt.Errorf("failed to register with registry: %w", err)
	}
	
	// Publish registration event
	event := events.NewEventBuilder("worker.registered").
		WithAggregateID(worker.ID).
		WithPayload("address", worker.Address).
		WithPayload("capacity", worker.Capacity).
		Build()
	
	c.eventBus.Publish(ctx, event)
	
	c.logger.Info("Worker registered",
		"workerId", worker.ID,
		"address", worker.Address,
		"capacity", worker.Capacity,
	)
	
	return nil
}

// UnregisterWorker unregisters a worker
func (c *Coordinator) UnregisterWorker(ctx context.Context, workerID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	worker, exists := c.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}
	
	// Mark as draining
	worker.Status = WorkerStatusDraining
	
	// Reassign work
	c.reassignWorkFromWorker(ctx, workerID)
	
	// Remove from workers
	delete(c.workers, workerID)
	
	// Remove from registry
	c.registry.Unregister(ctx, workerID)
	
	// Publish event
	event := events.NewEventBuilder("worker.unregistered").
		WithAggregateID(workerID).
		Build()
	
	c.eventBus.Publish(ctx, event)
	
	c.logger.Info("Worker unregistered", "workerId", workerID)
	
	return nil
}

// UpdateWorkerHeartbeat updates the heartbeat for a worker
func (c *Coordinator) UpdateWorkerHeartbeat(ctx context.Context, workerID string, metrics WorkerMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	worker, exists := c.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}
	
	worker.mu.Lock()
	worker.LastHeartbeat = time.Now()
	worker.CurrentLoad = metrics.CurrentLoad
	worker.ExecutionsCompleted = metrics.ExecutionsCompleted
	worker.ExecutionsFailed = metrics.ExecutionsFailed
	worker.AverageExecutionTime = metrics.AverageExecutionTime
	
	// Update status based on health
	if worker.Status == WorkerStatusUnhealthy && metrics.Healthy {
		worker.Status = WorkerStatusActive
		c.logger.Info("Worker recovered", "workerId", workerID)
	}
	worker.mu.Unlock()
	
	return nil
}

// healthCheckLoop performs periodic health checks on workers
func (c *Coordinator) healthCheckLoop(ctx context.Context) {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.healthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck checks the health of all workers
func (c *Coordinator) performHealthCheck(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	unhealthyThreshold := 30 * time.Second
	offlineThreshold := 60 * time.Second
	
	for _, worker := range c.workers {
		worker.mu.Lock()
		timeSinceHeartbeat := now.Sub(worker.LastHeartbeat)
		
		switch {
		case timeSinceHeartbeat > offlineThreshold:
			if worker.Status != WorkerStatusOffline {
				worker.Status = WorkerStatusOffline
				c.logger.Warn("Worker offline", "workerId", worker.ID, "lastSeen", timeSinceHeartbeat)
				
				// Reassign work
				go c.reassignWorkFromWorker(ctx, worker.ID)
			}
			
		case timeSinceHeartbeat > unhealthyThreshold:
			if worker.Status == WorkerStatusActive {
				worker.Status = WorkerStatusUnhealthy
				c.logger.Warn("Worker unhealthy", "workerId", worker.ID, "lastSeen", timeSinceHeartbeat)
			}
			
		default:
			// Worker is healthy
			if worker.Status == WorkerStatusUnhealthy {
				worker.Status = WorkerStatusActive
				c.logger.Info("Worker recovered", "workerId", worker.ID)
			}
		}
		
		worker.mu.Unlock()
	}
}

// rebalanceLoop performs periodic rebalancing
func (c *Coordinator) rebalanceLoop(ctx context.Context) {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.rebalanceInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.performRebalance(ctx)
		}
	}
}

// performRebalance rebalances work across workers
func (c *Coordinator) performRebalance(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Calculate average load
	totalCapacity := 0
	totalLoad := 0
	activeWorkers := 0
	
	for _, worker := range c.workers {
		if worker.Status == WorkerStatusActive {
			totalCapacity += worker.Capacity
			totalLoad += worker.CurrentLoad
			activeWorkers++
		}
	}
	
	if activeWorkers == 0 {
		return
	}
	
	averageLoadPercentage := float64(totalLoad) / float64(totalCapacity)
	
	// Identify overloaded and underloaded workers
	var overloaded, underloaded []*WorkerNode
	
	for _, worker := range c.workers {
		if worker.Status != WorkerStatusActive {
			continue
		}
		
		workerLoadPercentage := float64(worker.CurrentLoad) / float64(worker.Capacity)
		
		if workerLoadPercentage > averageLoadPercentage*1.2 {
			overloaded = append(overloaded, worker)
		} else if workerLoadPercentage < averageLoadPercentage*0.8 {
			underloaded = append(underloaded, worker)
		}
	}
	
	// Rebalance if needed
	if len(overloaded) > 0 && len(underloaded) > 0 {
		c.logger.Info("Rebalancing work",
			"overloaded", len(overloaded),
			"underloaded", len(underloaded),
			"averageLoad", averageLoadPercentage,
		)
		
		// Move work from overloaded to underloaded workers
		// This is simplified - in production, would move actual executions
		for _, overWorker := range overloaded {
			for _, underWorker := range underloaded {
				if overWorker.CurrentLoad > overWorker.Capacity/2 &&
					underWorker.CurrentLoad < underWorker.Capacity/2 {
					// Simulate moving work
					overWorker.CurrentLoad--
					underWorker.CurrentLoad++
				}
			}
		}
	}
}

// reassignWorkFromWorker reassigns work from a specific worker
func (c *Coordinator) reassignWorkFromWorker(ctx context.Context, workerID string) {
	// Find executions assigned to this worker
	var executionsToReassign []string
	
	for execID, assignedWorkerID := range c.partitions {
		if assignedWorkerID == workerID {
			executionsToReassign = append(executionsToReassign, execID)
		}
	}
	
	c.logger.Info("Reassigning work from worker",
		"workerId", workerID,
		"executions", len(executionsToReassign),
	)
	
	// Reassign each execution
	for _, execID := range executionsToReassign {
		delete(c.partitions, execID)
		
		// Find new worker
		worker := c.selectWorker(WorkRequirements{
			SelectionStrategy: SelectionStrategyLeastLoaded,
		})
		
		if worker != nil {
			c.partitions[execID] = worker.ID
			worker.CurrentLoad++
			
			// Publish reassignment event
			event := events.NewEventBuilder("work.reassigned").
				WithAggregateID(execID).
				WithPayload("fromWorkerId", workerID).
				WithPayload("toWorkerId", worker.ID).
				Build()
			
			c.eventBus.Publish(ctx, event)
		} else {
			c.logger.Error("Failed to reassign work - no available workers", "executionId", execID)
		}
	}
}

// metricsLoop reports metrics periodically
func (c *Coordinator) metricsLoop(ctx context.Context) {
	defer c.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.reportMetrics(ctx)
		}
	}
}

// reportMetrics reports coordinator metrics
func (c *Coordinator) reportMetrics(ctx context.Context) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	activeWorkers := 0
	totalCapacity := 0
	totalLoad := 0
	
	for _, worker := range c.workers {
		if worker.Status == WorkerStatusActive {
			activeWorkers++
			totalCapacity += worker.Capacity
			totalLoad += worker.CurrentLoad
		}
	}
	
	metrics := CoordinatorMetrics{
		TotalWorkers:        len(c.workers),
		ActiveWorkers:       activeWorkers,
		TotalCapacity:       totalCapacity,
		CurrentLoad:         totalLoad,
		PartitionedWork:     len(c.partitions),
		TotalExecutions:     atomic.LoadInt64(&c.totalExecutions),
		DistributedWork:     atomic.LoadInt64(&c.distributedWork),
		FailedDistributions: atomic.LoadInt64(&c.failedDistributions),
	}
	
	// Publish metrics event
	event := events.NewEventBuilder("coordinator.metrics").
		WithPayload("metrics", metrics).
		Build()
	
	c.eventBus.Publish(ctx, event)
	
	c.logger.Info("Coordinator metrics",
		"activeWorkers", metrics.ActiveWorkers,
		"totalCapacity", metrics.TotalCapacity,
		"currentLoad", metrics.CurrentLoad,
		"partitions", metrics.PartitionedWork,
	)
}

// subscribeToEvents subscribes to relevant events
func (c *Coordinator) subscribeToEvents(ctx context.Context) error {
	// Subscribe to worker lifecycle events
	if err := c.eventBus.Subscribe("worker.heartbeat", c.handleWorkerHeartbeat); err != nil {
		return err
	}
	
	if err := c.eventBus.Subscribe("work.completed", c.handleWorkCompleted); err != nil {
		return err
	}
	
	return nil
}

// handleWorkerHeartbeat handles worker heartbeat events
func (c *Coordinator) handleWorkerHeartbeat(ctx context.Context, event events.Event) error {
	workerID, _ := event.Payload["workerId"].(string)
	
	metricsData, _ := event.Payload["metrics"].(map[string]interface{})
	metrics := WorkerMetrics{
		CurrentLoad:          int(metricsData["currentLoad"].(float64)),
		ExecutionsCompleted:  int64(metricsData["executionsCompleted"].(float64)),
		ExecutionsFailed:     int64(metricsData["executionsFailed"].(float64)),
		Healthy:              metricsData["healthy"].(bool),
	}
	
	return c.UpdateWorkerHeartbeat(ctx, workerID, metrics)
}

// handleWorkCompleted handles work completion events
func (c *Coordinator) handleWorkCompleted(ctx context.Context, event events.Event) error {
	executionID, _ := event.Payload["executionId"].(string)
	workerID, _ := event.Payload["workerId"].(string)
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Remove from partitions
	delete(c.partitions, executionID)
	
	// Update worker load
	if worker, exists := c.workers[workerID]; exists {
		worker.CurrentLoad--
		if worker.CurrentLoad < 0 {
			worker.CurrentLoad = 0
		}
	}
	
	atomic.AddInt64(&c.totalExecutions, 1)
	
	return nil
}

// loadWorkers loads existing workers from the registry
func (c *Coordinator) loadWorkers(ctx context.Context) error {
	workers, err := c.registry.ListWorkers(ctx)
	if err != nil {
		return err
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for _, worker := range workers {
		c.workers[worker.ID] = worker
	}
	
	c.logger.Info("Loaded workers from registry", "count", len(workers))
	return nil
}

// GetWorkerStatus returns the status of all workers
func (c *Coordinator) GetWorkerStatus() []WorkerNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	workers := make([]WorkerNode, 0, len(c.workers))
	for _, worker := range c.workers {
		workers = append(workers, *worker)
	}
	
	return workers
}

// WorkRequirements defines requirements for work assignment
type WorkRequirements struct {
	RequiresTags      []string
	RequiredCapacity  int
	SelectionStrategy SelectionStrategy
	AffinityKey       string
}

// SelectionStrategy defines how workers are selected
type SelectionStrategy string

const (
	SelectionStrategyLeastLoaded SelectionStrategy = "least_loaded"
	SelectionStrategyRoundRobin  SelectionStrategy = "round_robin"
	SelectionStrategyRandom       SelectionStrategy = "random"
	SelectionStrategyAffinity     SelectionStrategy = "affinity"
)

// WorkerMetrics contains metrics for a worker
type WorkerMetrics struct {
	CurrentLoad          int
	ExecutionsCompleted  int64
	ExecutionsFailed     int64
	AverageExecutionTime time.Duration
	Healthy              bool
}

// CoordinatorMetrics contains metrics for the coordinator
type CoordinatorMetrics struct {
	TotalWorkers        int   `json:"totalWorkers"`
	ActiveWorkers       int   `json:"activeWorkers"`
	TotalCapacity       int   `json:"totalCapacity"`
	CurrentLoad         int   `json:"currentLoad"`
	PartitionedWork     int   `json:"partitionedWork"`
	TotalExecutions     int64 `json:"totalExecutions"`
	DistributedWork     int64 `json:"distributedWork"`
	FailedDistributions int64 `json:"failedDistributions"`
}
