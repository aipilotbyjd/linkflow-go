package distributed

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/logger"
)

// WorkDistributor distributes work across workers using various strategies
type WorkDistributor struct {
	coordinator *Coordinator
	strategy    PartitionStrategy
	partitions  map[string]*Partition
	mu          sync.RWMutex
	logger      logger.Logger
}

// Partition represents a work partition
type Partition struct {
	ID            string
	WorkerID      string
	WorkItems     []string
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	mu            sync.RWMutex
}

// PartitionStrategy defines how work is partitioned
type PartitionStrategy interface {
	Partition(workItems []string, workers []*WorkerNode) map[string][]string
	Rebalance(currentPartitions map[string]*Partition, workers []*WorkerNode) map[string]*Partition
}

// NewWorkDistributor creates a new work distributor
func NewWorkDistributor(coordinator *Coordinator, logger logger.Logger) *WorkDistributor {
	return &WorkDistributor{
		coordinator: coordinator,
		strategy:    &ConsistentHashStrategy{replicas: 100},
		partitions:  make(map[string]*Partition),
		logger:      logger,
	}
}

// DistributeWork distributes work items across available workers
func (wd *WorkDistributor) DistributeWork(ctx context.Context, workItems []string) error {
	workers := wd.coordinator.GetWorkerStatus()

	// Filter active workers
	activeWorkers := make([]*WorkerNode, 0)
	for _, worker := range workers {
		if worker.Status == WorkerStatusActive {
			activeWorkers = append(activeWorkers, worker)
		}
	}

	if len(activeWorkers) == 0 {
		return fmt.Errorf("no active workers available")
	}

	// Partition work
	assignments := wd.strategy.Partition(workItems, activeWorkers)

	// Create partitions
	wd.mu.Lock()
	defer wd.mu.Unlock()

	for workerID, items := range assignments {
		partition := &Partition{
			ID:            fmt.Sprintf("partition-%d", time.Now().UnixNano()),
			WorkerID:      workerID,
			WorkItems:     items,
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		}

		wd.partitions[partition.ID] = partition

		wd.logger.Info("Created partition",
			"partitionId", partition.ID,
			"workerId", workerID,
			"items", len(items),
		)
	}

	return nil
}

// GetPartitionForWork returns the partition for a specific work item
func (wd *WorkDistributor) GetPartitionForWork(workID string) *Partition {
	wd.mu.RLock()
	defer wd.mu.RUnlock()

	for _, partition := range wd.partitions {
		partition.mu.RLock()
		for _, item := range partition.WorkItems {
			if item == workID {
				partition.mu.RUnlock()
				return partition
			}
		}
		partition.mu.RUnlock()
	}

	return nil
}

// Rebalance rebalances partitions across workers
func (wd *WorkDistributor) Rebalance(ctx context.Context) error {
	workers := wd.coordinator.GetWorkerStatus()

	// Filter active workers
	activeWorkers := make([]*WorkerNode, 0)
	for _, worker := range workers {
		if worker.Status == WorkerStatusActive {
			activeWorkers = append(activeWorkers, worker)
		}
	}

	if len(activeWorkers) == 0 {
		return fmt.Errorf("no active workers for rebalancing")
	}

	wd.mu.Lock()
	defer wd.mu.Unlock()

	// Rebalance existing partitions
	newPartitions := wd.strategy.Rebalance(wd.partitions, activeWorkers)

	// Update partitions
	wd.partitions = newPartitions

	wd.logger.Info("Rebalanced partitions",
		"partitions", len(newPartitions),
		"workers", len(activeWorkers),
	)

	return nil
}

// ConsistentHashStrategy implements consistent hashing for work distribution
type ConsistentHashStrategy struct {
	replicas int
	ring     map[uint32]string
	mu       sync.RWMutex
}

// Partition partitions work using consistent hashing
func (chs *ConsistentHashStrategy) Partition(workItems []string, workers []*WorkerNode) map[string][]string {
	// Build hash ring
	chs.buildRing(workers)

	assignments := make(map[string][]string)

	// Assign each work item
	for _, item := range workItems {
		workerID := chs.getWorkerForItem(item)
		if workerID != "" {
			assignments[workerID] = append(assignments[workerID], item)
		}
	}

	return assignments
}

// Rebalance rebalances partitions using consistent hashing
func (chs *ConsistentHashStrategy) Rebalance(currentPartitions map[string]*Partition, workers []*WorkerNode) map[string]*Partition {
	// Build new hash ring
	chs.buildRing(workers)

	// Collect all work items
	allItems := []string{}
	for _, partition := range currentPartitions {
		partition.mu.RLock()
		allItems = append(allItems, partition.WorkItems...)
		partition.mu.RUnlock()
	}

	// Redistribute items
	assignments := chs.Partition(allItems, workers)

	// Create new partitions
	newPartitions := make(map[string]*Partition)

	for workerID, items := range assignments {
		partition := &Partition{
			ID:            fmt.Sprintf("partition-%d", time.Now().UnixNano()),
			WorkerID:      workerID,
			WorkItems:     items,
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		}

		newPartitions[partition.ID] = partition
	}

	return newPartitions
}

// buildRing builds the consistent hash ring
func (chs *ConsistentHashStrategy) buildRing(workers []*WorkerNode) {
	chs.mu.Lock()
	defer chs.mu.Unlock()

	chs.ring = make(map[uint32]string)

	for _, worker := range workers {
		for i := 0; i < chs.replicas; i++ {
			hash := chs.hash(fmt.Sprintf("%s:%d", worker.ID, i))
			chs.ring[hash] = worker.ID
		}
	}
}

// getWorkerForItem gets the worker for a specific item
func (chs *ConsistentHashStrategy) getWorkerForItem(item string) string {
	chs.mu.RLock()
	defer chs.mu.RUnlock()

	if len(chs.ring) == 0 {
		return ""
	}

	hash := chs.hash(item)

	// Find the first node with hash >= item hash
	var keys []uint32
	for k := range chs.ring {
		keys = append(keys, k)
	}

	// Simple linear search (in production, use sorted keys)
	var selected uint32
	minDiff := uint32(^uint32(0))

	for _, key := range keys {
		if key >= hash {
			diff := key - hash
			if diff < minDiff {
				minDiff = diff
				selected = key
			}
		}
	}

	// If no key >= hash, wrap around to smallest key
	if selected == 0 && len(keys) > 0 {
		selected = keys[0]
		for _, key := range keys {
			if key < selected {
				selected = key
			}
		}
	}

	return chs.ring[selected]
}

// hash generates a hash for a string
func (chs *ConsistentHashStrategy) hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// RangePartitionStrategy implements range-based partitioning
type RangePartitionStrategy struct {
	rangeSize int
}

// NewRangePartitionStrategy creates a new range partition strategy
func NewRangePartitionStrategy(rangeSize int) *RangePartitionStrategy {
	if rangeSize <= 0 {
		rangeSize = 100
	}

	return &RangePartitionStrategy{
		rangeSize: rangeSize,
	}
}

// Partition partitions work into ranges
func (rps *RangePartitionStrategy) Partition(workItems []string, workers []*WorkerNode) map[string][]string {
	if len(workers) == 0 {
		return nil
	}

	assignments := make(map[string][]string)
	itemsPerWorker := len(workItems) / len(workers)
	if itemsPerWorker == 0 {
		itemsPerWorker = 1
	}

	workerIndex := 0
	currentBatch := []string{}

	for _, item := range workItems {
		currentBatch = append(currentBatch, item)

		if len(currentBatch) >= itemsPerWorker && workerIndex < len(workers)-1 {
			assignments[workers[workerIndex].ID] = currentBatch
			currentBatch = []string{}
			workerIndex++
		}
	}

	// Assign remaining items to last worker
	if len(currentBatch) > 0 && workerIndex < len(workers) {
		assignments[workers[workerIndex].ID] = currentBatch
	}

	return assignments
}

// Rebalance rebalances partitions using range partitioning
func (rps *RangePartitionStrategy) Rebalance(currentPartitions map[string]*Partition, workers []*WorkerNode) map[string]*Partition {
	// Collect all work items
	allItems := []string{}
	for _, partition := range currentPartitions {
		partition.mu.RLock()
		allItems = append(allItems, partition.WorkItems...)
		partition.mu.RUnlock()
	}

	// Redistribute items
	assignments := rps.Partition(allItems, workers)

	// Create new partitions
	newPartitions := make(map[string]*Partition)

	for workerID, items := range assignments {
		partition := &Partition{
			ID:            fmt.Sprintf("partition-%d", time.Now().UnixNano()),
			WorkerID:      workerID,
			WorkItems:     items,
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		}

		newPartitions[partition.ID] = partition
	}

	return newPartitions
}

// HashPartitionStrategy implements hash-based partitioning
type HashPartitionStrategy struct{}

// Partition partitions work using hash-based distribution
func (hps *HashPartitionStrategy) Partition(workItems []string, workers []*WorkerNode) map[string][]string {
	if len(workers) == 0 {
		return nil
	}

	assignments := make(map[string][]string)

	for _, item := range workItems {
		// Hash the item to determine worker
		h := fnv.New32a()
		h.Write([]byte(item))
		hash := h.Sum32()

		workerIndex := int(hash % uint32(len(workers)))
		workerID := workers[workerIndex].ID

		assignments[workerID] = append(assignments[workerID], item)
	}

	return assignments
}

// Rebalance rebalances partitions using hash partitioning
func (hps *HashPartitionStrategy) Rebalance(currentPartitions map[string]*Partition, workers []*WorkerNode) map[string]*Partition {
	// Collect all work items
	allItems := []string{}
	for _, partition := range currentPartitions {
		partition.mu.RLock()
		allItems = append(allItems, partition.WorkItems...)
		partition.mu.RUnlock()
	}

	// Redistribute items
	assignments := hps.Partition(allItems, workers)

	// Create new partitions
	newPartitions := make(map[string]*Partition)

	for workerID, items := range assignments {
		partition := &Partition{
			ID:            fmt.Sprintf("partition-%d", time.Now().UnixNano()),
			WorkerID:      workerID,
			WorkItems:     items,
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		}

		newPartitions[partition.ID] = partition
	}

	return newPartitions
}
