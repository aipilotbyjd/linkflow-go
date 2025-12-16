package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Metrics keys
const (
	MetricExecutions      = "executions"
	MetricSuccessRate     = "success_rate"
	MetricAverageRuntime  = "avg_runtime"
	MetricErrorRate       = "error_rate"
	MetricThroughput      = "throughput"
	MetricActiveWorkflows = "active_workflows"
)

// WorkflowStats represents workflow statistics
type WorkflowStats struct {
	WorkflowID        string        `json:"workflowId" gorm:"primaryKey"`
	TotalExecutions   int64         `json:"totalExecutions"`
	SuccessfulRuns    int64         `json:"successfulRuns"`
	FailedRuns        int64         `json:"failedRuns"`
	CancelledRuns     int64         `json:"cancelledRuns"`
	AverageRuntime    time.Duration `json:"averageRuntime"`
	MinRuntime        time.Duration `json:"minRuntime"`
	MaxRuntime        time.Duration `json:"maxRuntime"`
	LastExecution     *time.Time    `json:"lastExecution"`
	LastSuccessfulRun *time.Time    `json:"lastSuccessfulRun"`
	LastFailedRun     *time.Time    `json:"lastFailedRun"`
	ErrorRate         float64       `json:"errorRate"`
	SuccessRate       float64       `json:"successRate"`
	ThroughputPerHour float64       `json:"throughputPerHour"`
	ThroughputPerDay  float64       `json:"throughputPerDay"`
	CommonErrors      []ErrorStats  `json:"commonErrors" gorm:"serializer:json"`
	NodeStatistics    []NodeStats   `json:"nodeStatistics" gorm:"serializer:json"`
	UpdatedAt         time.Time     `json:"updatedAt"`
}

// ErrorStats represents error statistics
type ErrorStats struct {
	ErrorType string    `json:"errorType"`
	Count     int64     `json:"count"`
	LastSeen  time.Time `json:"lastSeen"`
	NodeID    string    `json:"nodeId,omitempty"`
}

// NodeStats represents node-level statistics
type NodeStats struct {
	NodeID         string        `json:"nodeId"`
	NodeName       string        `json:"nodeName"`
	Executions     int64         `json:"executions"`
	Failures       int64         `json:"failures"`
	AverageRuntime time.Duration `json:"averageRuntime"`
	ErrorRate      float64       `json:"errorRate"`
}

// TimeSeriesData represents time-series statistics
type TimeSeriesData struct {
	Timestamp  time.Time              `json:"timestamp"`
	Executions int64                  `json:"executions"`
	Successes  int64                  `json:"successes"`
	Failures   int64                  `json:"failures"`
	AvgRuntime time.Duration          `json:"avgRuntime"`
	Metrics    map[string]interface{} `json:"metrics"`
}

// StatsCollector collects and aggregates workflow statistics
type StatsCollector struct {
	db            *database.DB
	redis         *redis.Client
	logger        logger.Logger
	bufferSize    int
	flushInterval time.Duration
	buffer        map[string]*WorkflowStats
	bufferMu      sync.RWMutex
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewStatsCollector creates a new statistics collector
func NewStatsCollector(db *database.DB, redis *redis.Client, logger logger.Logger) *StatsCollector {
	return &StatsCollector{
		db:            db,
		redis:         redis,
		logger:        logger,
		bufferSize:    100,
		flushInterval: 30 * time.Second,
		buffer:        make(map[string]*WorkflowStats),
		stopCh:        make(chan struct{}),
	}
}

// Start starts the statistics collector
func (sc *StatsCollector) Start(ctx context.Context) {
	sc.wg.Add(1)
	go sc.flushLoop(ctx)
	sc.logger.Info("Statistics collector started")
}

// Stop stops the statistics collector
func (sc *StatsCollector) Stop() {
	close(sc.stopCh)
	sc.wg.Wait()
	sc.flush(context.Background())
	sc.logger.Info("Statistics collector stopped")
}

// RecordExecution records a workflow execution
func (sc *StatsCollector) RecordExecution(ctx context.Context, execution *workflow.WorkflowExecution) error {
	sc.bufferMu.Lock()
	defer sc.bufferMu.Unlock()

	// Get or create stats for this workflow
	stats, exists := sc.buffer[execution.WorkflowID]
	if !exists {
		stats = &WorkflowStats{
			WorkflowID:     execution.WorkflowID,
			CommonErrors:   []ErrorStats{},
			NodeStatistics: []NodeStats{},
		}
		sc.buffer[execution.WorkflowID] = stats
	}

	// Update statistics
	stats.TotalExecutions++
	now := time.Now()
	stats.LastExecution = &now

	// Calculate runtime if finished
	if execution.FinishedAt != nil {
		runtime := execution.ExecutionTime

		// Update runtime statistics
		if stats.AverageRuntime == 0 {
			stats.AverageRuntime = time.Duration(runtime)
			stats.MinRuntime = time.Duration(runtime)
			stats.MaxRuntime = time.Duration(runtime)
		} else {
			// Calculate new average
			totalRuntime := stats.AverageRuntime * time.Duration(stats.TotalExecutions-1)
			stats.AverageRuntime = (totalRuntime + time.Duration(runtime)) / time.Duration(stats.TotalExecutions)

			// Update min/max
			if time.Duration(runtime) < stats.MinRuntime {
				stats.MinRuntime = time.Duration(runtime)
			}
			if time.Duration(runtime) > stats.MaxRuntime {
				stats.MaxRuntime = time.Duration(runtime)
			}
		}

		// Update status-based counters
		switch execution.Status {
		case string(workflow.ExecutionCompleted):
			stats.SuccessfulRuns++
			stats.LastSuccessfulRun = &now
		case string(workflow.ExecutionFailed):
			stats.FailedRuns++
			stats.LastFailedRun = &now
			sc.recordError(stats, execution.Error)
		case string(workflow.ExecutionCancelled):
			stats.CancelledRuns++
		}
	}

	// Calculate rates
	stats.SuccessRate = float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions) * 100
	stats.ErrorRate = float64(stats.FailedRuns) / float64(stats.TotalExecutions) * 100

	// Store in Redis for real-time access
	sc.storeInRedis(ctx, stats)

	// Check if buffer should be flushed
	if len(sc.buffer) >= sc.bufferSize {
		go sc.flush(ctx)
	}

	return nil
}

// RecordNodeExecution records a node execution
func (sc *StatsCollector) RecordNodeExecution(ctx context.Context, nodeExec *workflow.NodeExecution) error {
	sc.bufferMu.Lock()
	defer sc.bufferMu.Unlock()

	// Extract workflow ID from execution ID pattern (simplified)
	workflowID := extractWorkflowID(nodeExec.ExecutionID)

	stats, exists := sc.buffer[workflowID]
	if !exists {
		stats = &WorkflowStats{
			WorkflowID:     workflowID,
			NodeStatistics: []NodeStats{},
		}
		sc.buffer[workflowID] = stats
	}

	// Update node statistics
	var nodeStats *NodeStats
	for i := range stats.NodeStatistics {
		if stats.NodeStatistics[i].NodeID == nodeExec.NodeID {
			nodeStats = &stats.NodeStatistics[i]
			break
		}
	}

	if nodeStats == nil {
		stats.NodeStatistics = append(stats.NodeStatistics, NodeStats{
			NodeID: nodeExec.NodeID,
		})
		nodeStats = &stats.NodeStatistics[len(stats.NodeStatistics)-1]
	}

	nodeStats.Executions++

	// Update based on status
	if nodeExec.Status == string(workflow.NodeExecutionFailed) {
		nodeStats.Failures++
	}

	// Calculate runtime if finished
	if nodeExec.FinishedAt != nil {
		runtime := nodeExec.FinishedAt.Sub(nodeExec.StartedAt)
		if nodeStats.AverageRuntime == 0 {
			nodeStats.AverageRuntime = runtime
		} else {
			totalRuntime := nodeStats.AverageRuntime * time.Duration(nodeStats.Executions-1)
			nodeStats.AverageRuntime = (totalRuntime + runtime) / time.Duration(nodeStats.Executions)
		}
	}

	// Calculate error rate
	if nodeStats.Executions > 0 {
		nodeStats.ErrorRate = float64(nodeStats.Failures) / float64(nodeStats.Executions) * 100
	}

	return nil
}

// GetWorkflowStats retrieves statistics for a workflow
func (sc *StatsCollector) GetWorkflowStats(ctx context.Context, workflowID string) (*WorkflowStats, error) {
	// Check buffer first
	sc.bufferMu.RLock()
	if stats, ok := sc.buffer[workflowID]; ok {
		sc.bufferMu.RUnlock()
		return stats, nil
	}
	sc.bufferMu.RUnlock()

	// Check Redis cache
	if stats := sc.getFromRedis(ctx, workflowID); stats != nil {
		return stats, nil
	}

	// Load from database
	var stats WorkflowStats
	err := sc.db.WithContext(ctx).Where("workflow_id = ?", workflowID).First(&stats).Error
	if err == gorm.ErrRecordNotFound {
		// Return empty stats
		return &WorkflowStats{
			WorkflowID: workflowID,
		}, nil
	}

	return &stats, err
}

// GetAggregatedStats gets aggregated statistics
func (sc *StatsCollector) GetAggregatedStats(ctx context.Context, userID string, period string) (map[string]interface{}, error) {
	endTime := time.Now()
	var startTime time.Time

	switch period {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-24 * time.Hour)
	}

	// Query aggregated stats from database
	var result struct {
		TotalExecutions int64
		SuccessfulRuns  int64
		FailedRuns      int64
		AverageRuntime  float64
		ActiveWorkflows int64
	}

	query := sc.db.WithContext(ctx).Model(&WorkflowStats{}).
		Select(`
			SUM(total_executions) as total_executions,
			SUM(successful_runs) as successful_runs,
			SUM(failed_runs) as failed_runs,
			AVG(average_runtime) as average_runtime,
			COUNT(DISTINCT workflow_id) as active_workflows
		`).
		Where("updated_at BETWEEN ? AND ?", startTime, endTime)

	if userID != "" {
		// Would need to join with workflow table for user filtering
		query = query.Joins("JOIN workflows ON workflow_stats.workflow_id = workflows.id").
			Where("workflows.user_id = ?", userID)
	}

	if err := query.Scan(&result).Error; err != nil {
		return nil, err
	}

	// Calculate additional metrics
	successRate := float64(0)
	errorRate := float64(0)

	if result.TotalExecutions > 0 {
		successRate = float64(result.SuccessfulRuns) / float64(result.TotalExecutions) * 100
		errorRate = float64(result.FailedRuns) / float64(result.TotalExecutions) * 100
	}

	hoursDiff := endTime.Sub(startTime).Hours()
	throughputPerHour := float64(result.TotalExecutions) / hoursDiff

	return map[string]interface{}{
		"period":            period,
		"startTime":         startTime,
		"endTime":           endTime,
		"totalExecutions":   result.TotalExecutions,
		"successfulRuns":    result.SuccessfulRuns,
		"failedRuns":        result.FailedRuns,
		"averageRuntime":    time.Duration(result.AverageRuntime),
		"successRate":       successRate,
		"errorRate":         errorRate,
		"throughputPerHour": throughputPerHour,
		"activeWorkflows":   result.ActiveWorkflows,
	}, nil
}

// GetTimeSeries gets time-series data for a workflow
func (sc *StatsCollector) GetTimeSeries(ctx context.Context, workflowID string, interval string, period string) ([]TimeSeriesData, error) {
	// This would typically query from a time-series database
	// For now, we'll use Redis to store recent time-series data

	key := fmt.Sprintf("timeseries:%s:%s", workflowID, interval)
	data, err := sc.redis.Get(ctx, key).Result()
	if err != nil {
		return []TimeSeriesData{}, nil
	}

	var series []TimeSeriesData
	json.Unmarshal([]byte(data), &series)

	return series, nil
}

// flushLoop periodically flushes the buffer
func (sc *StatsCollector) flushLoop(ctx context.Context) {
	defer sc.wg.Done()

	ticker := time.NewTicker(sc.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.flush(ctx)
		case <-sc.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// flush writes buffered statistics to database
func (sc *StatsCollector) flush(ctx context.Context) {
	sc.bufferMu.Lock()
	defer sc.bufferMu.Unlock()

	if len(sc.buffer) == 0 {
		return
	}

	// Copy buffer
	toFlush := make([]*WorkflowStats, 0, len(sc.buffer))
	for _, stats := range sc.buffer {
		stats.UpdatedAt = time.Now()
		toFlush = append(toFlush, stats)
	}

	// Clear buffer
	sc.buffer = make(map[string]*WorkflowStats)

	// Write to database
	go func() {
		for _, stats := range toFlush {
			// Use upsert to update existing or create new
			sc.db.WithContext(ctx).Save(stats)
		}
		sc.logger.Debug("Flushed statistics to database", "count", len(toFlush))
	}()
}

// storeInRedis stores statistics in Redis for fast access
func (sc *StatsCollector) storeInRedis(ctx context.Context, stats *WorkflowStats) {
	key := fmt.Sprintf("stats:workflow:%s", stats.WorkflowID)
	data, _ := json.Marshal(stats)

	// Store with TTL
	sc.redis.Set(ctx, key, string(data), 1*time.Hour)
}

// getFromRedis retrieves statistics from Redis
func (sc *StatsCollector) getFromRedis(ctx context.Context, workflowID string) *WorkflowStats {
	key := fmt.Sprintf("stats:workflow:%s", workflowID)
	data, err := sc.redis.Get(ctx, key).Result()
	if err != nil {
		return nil
	}

	var stats WorkflowStats
	json.Unmarshal([]byte(data), &stats)
	return &stats
}

// recordError records error information
func (sc *StatsCollector) recordError(stats *WorkflowStats, errorMsg string) {
	if errorMsg == "" {
		return
	}

	// Find or create error stats
	var errorStats *ErrorStats
	for i := range stats.CommonErrors {
		if stats.CommonErrors[i].ErrorType == errorMsg {
			errorStats = &stats.CommonErrors[i]
			break
		}
	}

	if errorStats == nil {
		stats.CommonErrors = append(stats.CommonErrors, ErrorStats{
			ErrorType: errorMsg,
		})
		errorStats = &stats.CommonErrors[len(stats.CommonErrors)-1]
	}

	errorStats.Count++
	errorStats.LastSeen = time.Now()

	// Keep only top 10 errors
	if len(stats.CommonErrors) > 10 {
		stats.CommonErrors = stats.CommonErrors[:10]
	}
}

// extractWorkflowID extracts workflow ID from execution ID (simplified)
func extractWorkflowID(executionID string) string {
	// In a real implementation, you'd have a proper mapping
	return "workflow_" + executionID
}
