package cost

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/logger"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// UsageTracker tracks resource usage for executions
type UsageTracker struct {
	mu            sync.RWMutex
	usageData     map[string]*ResourceUsage
	activeTracking map[string]*TrackingSession
	logger        logger.Logger
	
	// System monitors
	cpuMonitor    *CPUMonitor
	memoryMonitor *MemoryMonitor
	networkMonitor *NetworkMonitor
	
	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// TrackingSession represents an active tracking session
type TrackingSession struct {
	ExecutionID      string
	StartTime        time.Time
	LastUpdate       time.Time
	
	// Accumulated usage
	CPUSeconds       float64
	MemoryByteSeconds float64
	NetworkBytes     int64
	StorageBytes     int64
	APICallCount     int
	DatabaseQueries  int
	
	// Snapshots
	InitialCPU       float64
	InitialMemory    uint64
	InitialNetwork   uint64
}

// CPUMonitor monitors CPU usage
type CPUMonitor struct {
	logger logger.Logger
}

// MemoryMonitor monitors memory usage
type MemoryMonitor struct {
	logger logger.Logger
}

// NetworkMonitor monitors network usage
type NetworkMonitor struct {
	logger logger.Logger
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker(logger logger.Logger) *UsageTracker {
	return &UsageTracker{
		usageData:      make(map[string]*ResourceUsage),
		activeTracking: make(map[string]*TrackingSession),
		logger:         logger,
		cpuMonitor:     &CPUMonitor{logger: logger},
		memoryMonitor:  &MemoryMonitor{logger: logger},
		networkMonitor: &NetworkMonitor{logger: logger},
		stopCh:         make(chan struct{}),
	}
}

// Start starts the usage tracker
func (t *UsageTracker) Start(ctx context.Context) error {
	t.logger.Info("Starting usage tracker")
	
	// Start monitoring
	t.wg.Add(1)
	go t.monitorLoop(ctx)
	
	return nil
}

// Stop stops the usage tracker
func (t *UsageTracker) Stop(ctx context.Context) error {
	t.logger.Info("Stopping usage tracker")
	
	close(t.stopCh)
	
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		t.logger.Info("Usage tracker stopped")
	case <-ctx.Done():
		t.logger.Warn("Usage tracker stop timeout")
	}
	
	return nil
}

// StartTracking starts tracking resource usage for an execution
func (t *UsageTracker) StartTracking(executionID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if _, exists := t.activeTracking[executionID]; exists {
		return fmt.Errorf("already tracking execution: %s", executionID)
	}
	
	// Get initial resource snapshot
	cpuPercent, _ := t.cpuMonitor.GetCPUUsage()
	memoryUsage, _ := t.memoryMonitor.GetMemoryUsage()
	networkStats, _ := t.networkMonitor.GetNetworkStats()
	
	session := &TrackingSession{
		ExecutionID:    executionID,
		StartTime:      time.Now(),
		LastUpdate:     time.Now(),
		InitialCPU:     cpuPercent,
		InitialMemory:  memoryUsage,
		InitialNetwork: networkStats,
	}
	
	t.activeTracking[executionID] = session
	
	t.logger.Info("Started tracking resource usage", "executionId", executionID)
	
	return nil
}

// StopTracking stops tracking resource usage for an execution
func (t *UsageTracker) StopTracking(executionID string) (*ResourceUsage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	session, exists := t.activeTracking[executionID]
	if !exists {
		return nil, fmt.Errorf("not tracking execution: %s", executionID)
	}
	
	// Get final resource snapshot
	finalCPU, _ := t.cpuMonitor.GetCPUUsage()
	finalMemory, _ := t.memoryMonitor.GetMemoryUsage()
	finalNetwork, _ := t.networkMonitor.GetNetworkStats()
	
	// Calculate total usage
	duration := time.Since(session.StartTime)
	
	usage := &ResourceUsage{
		ExecutionID:     executionID,
		ComputeTime:     duration,
		MemoryBytes:     int64(session.MemoryByteSeconds / duration.Seconds()),
		NetworkBytes:    finalNetwork - int64(session.InitialNetwork),
		StorageBytes:    session.StorageBytes,
		APICallCount:    session.APICallCount,
		DatabaseQueries: session.DatabaseQueries,
	}
	
	// Store usage data
	t.usageData[executionID] = usage
	
	// Remove from active tracking
	delete(t.activeTracking, executionID)
	
	t.logger.Info("Stopped tracking resource usage",
		"executionId", executionID,
		"duration", duration,
		"computeTime", usage.ComputeTime,
		"memoryBytes", usage.MemoryBytes,
	)
	
	return usage, nil
}

// UpdateUsage updates resource usage for an execution
func (t *UsageTracker) UpdateUsage(executionID string, update UsageUpdate) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	session, exists := t.activeTracking[executionID]
	if !exists {
		return fmt.Errorf("not tracking execution: %s", executionID)
	}
	
	// Update counters
	if update.APICallCount > 0 {
		session.APICallCount += update.APICallCount
	}
	
	if update.DatabaseQueries > 0 {
		session.DatabaseQueries += update.DatabaseQueries
	}
	
	if update.StorageBytes > 0 {
		session.StorageBytes += update.StorageBytes
	}
	
	session.LastUpdate = time.Now()
	
	return nil
}

// TrackUsage tracks resource usage (for completed executions)
func (t *UsageTracker) TrackUsage(executionID string, usage ResourceUsage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.usageData[executionID] = &usage
	
	return nil
}

// GetUsage gets resource usage for an execution
func (t *UsageTracker) GetUsage(executionID string) (*ResourceUsage, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	usage, exists := t.usageData[executionID]
	if !exists {
		// Check if still being tracked
		if session, tracking := t.activeTracking[executionID]; tracking {
			// Return current usage
			duration := time.Since(session.StartTime)
			return &ResourceUsage{
				ExecutionID:     executionID,
				ComputeTime:     duration,
				MemoryBytes:     int64(session.MemoryByteSeconds / duration.Seconds()),
				NetworkBytes:    session.NetworkBytes,
				StorageBytes:    session.StorageBytes,
				APICallCount:    session.APICallCount,
				DatabaseQueries: session.DatabaseQueries,
			}, nil
		}
		
		return nil, fmt.Errorf("usage data not found for execution: %s", executionID)
	}
	
	return usage, nil
}

// monitorLoop continuously monitors resource usage
func (t *UsageTracker) monitorLoop(ctx context.Context) {
	defer t.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.updateActiveTracking()
		}
	}
}

// updateActiveTracking updates usage for all active tracking sessions
func (t *UsageTracker) updateActiveTracking() {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	for executionID, session := range t.activeTracking {
		// Get current resource usage
		cpuPercent, _ := t.cpuMonitor.GetCPUUsage()
		memoryUsage, _ := t.memoryMonitor.GetMemoryUsage()
		
		// Calculate incremental usage
		timeDelta := time.Since(session.LastUpdate).Seconds()
		
		// Update CPU seconds
		session.CPUSeconds += cpuPercent * timeDelta / 100
		
		// Update memory byte-seconds
		session.MemoryByteSeconds += float64(memoryUsage) * timeDelta
		
		session.LastUpdate = time.Now()
		
		t.logger.Debug("Updated resource tracking",
			"executionId", executionID,
			"cpuSeconds", session.CPUSeconds,
			"memoryByteSeconds", session.MemoryByteSeconds,
		)
	}
}

// GetActiveTrackingSessions returns all active tracking sessions
func (t *UsageTracker) GetActiveTrackingSessions() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	sessions := make([]string, 0, len(t.activeTracking))
	for executionID := range t.activeTracking {
		sessions = append(sessions, executionID)
	}
	
	return sessions
}

// GetCPUUsage gets current CPU usage percentage
func (m *CPUMonitor) GetCPUUsage() (float64, error) {
	percent, err := cpu.Percent(0, false)
	if err != nil {
		return 0, err
	}
	
	if len(percent) > 0 {
		return percent[0], nil
	}
	
	return 0, nil
}

// GetMemoryUsage gets current memory usage in bytes
func (m *MemoryMonitor) GetMemoryUsage() (uint64, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	
	return vmStat.Used, nil
}

// GetNetworkStats gets current network statistics
func (m *NetworkMonitor) GetNetworkStats() (uint64, error) {
	stats, err := net.IOCounters(false)
	if err != nil {
		return 0, err
	}
	
	if len(stats) > 0 {
		return stats[0].BytesSent + stats[0].BytesRecv, nil
	}
	
	return 0, nil
}

// UsageUpdate represents an update to resource usage
type UsageUpdate struct {
	APICallCount    int
	DatabaseQueries int
	StorageBytes    int64
}

// UsageReport generates a usage report
type UsageReport struct {
	ExecutionID      string        `json:"execution_id"`
	Period           string        `json:"period"`
	TotalComputeTime time.Duration `json:"total_compute_time"`
	PeakMemoryBytes  int64         `json:"peak_memory_bytes"`
	TotalNetworkBytes int64        `json:"total_network_bytes"`
	TotalStorageBytes int64        `json:"total_storage_bytes"`
	TotalAPICalls    int           `json:"total_api_calls"`
	TotalDBQueries   int           `json:"total_db_queries"`
	
	// Hourly breakdown
	HourlyUsage []HourlyUsage `json:"hourly_usage"`
}

// HourlyUsage represents usage for an hour
type HourlyUsage struct {
	Hour         time.Time     `json:"hour"`
	ComputeTime  time.Duration `json:"compute_time"`
	MemoryBytes  int64         `json:"memory_bytes"`
	NetworkBytes int64         `json:"network_bytes"`
}

// GenerateReport generates a usage report for an execution
func (t *UsageTracker) GenerateReport(executionID string, period string) (*UsageReport, error) {
	usage, err := t.GetUsage(executionID)
	if err != nil {
		return nil, err
	}
	
	report := &UsageReport{
		ExecutionID:       executionID,
		Period:            period,
		TotalComputeTime:  usage.ComputeTime,
		PeakMemoryBytes:   usage.MemoryBytes,
		TotalNetworkBytes: usage.NetworkBytes,
		TotalStorageBytes: usage.StorageBytes,
		TotalAPICalls:     usage.APICallCount,
		TotalDBQueries:    usage.DatabaseQueries,
	}
	
	// In production, would generate hourly breakdown
	
	return report, nil
}
