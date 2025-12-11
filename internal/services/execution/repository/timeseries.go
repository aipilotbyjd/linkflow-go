package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"gorm.io/gorm"
)

// StateTransition represents a state change in execution
type StateTransition struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	ExecutionID string                 `json:"executionId" gorm:"not null;index"`
	FromState   string                 `json:"fromState"`
	ToState     string                 `json:"toState" gorm:"not null"`
	Timestamp   time.Time              `json:"timestamp" gorm:"not null;index"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"serializer:json"`
}

// ExecutionMetric represents performance metrics for an execution
type ExecutionMetric struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	ExecutionID string    `json:"executionId" gorm:"not null;index"`
	NodeID      string    `json:"nodeId" gorm:"index"`
	MetricType  string    `json:"metricType" gorm:"not null;index"`
	Value       float64   `json:"value" gorm:"not null"`
	Unit        string    `json:"unit"`
	Timestamp   time.Time `json:"timestamp" gorm:"not null;index"`
	Metadata    string    `json:"metadata" gorm:"type:jsonb"`
}

// TimeSeriesData represents aggregated time-series data
type TimeSeriesData struct {
	Timestamp time.Time              `json:"timestamp"`
	Count     int64                  `json:"count"`
	Value     float64                `json:"value"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// GetStateTransitions retrieves all state transitions for an execution
func (r *ExecutionRepository) GetStateTransitions(ctx context.Context, executionID string) ([]*StateTransition, error) {
	var transitions []*StateTransition
	err := r.db.WithContext(ctx).
		Where("execution_id = ?", executionID).
		Order("timestamp ASC").
		Find(&transitions).Error
	
	return transitions, err
}

// RecordMetric records a performance metric for an execution
func (r *ExecutionRepository) RecordMetric(ctx context.Context, metric *ExecutionMetric) error {
	metric.ID = uuid.New().String()
	metric.Timestamp = time.Now()
	return r.db.WithContext(ctx).Create(metric).Error
}

// RecordNodeMetrics records multiple metrics for a node execution
func (r *ExecutionRepository) RecordNodeMetrics(ctx context.Context, nodeExecID string, metrics map[string]float64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for metricType, value := range metrics {
			metric := &ExecutionMetric{
				ID:          uuid.New().String(),
				ExecutionID: nodeExecID,
				MetricType:  metricType,
				Value:       value,
				Timestamp:   time.Now(),
			}
			
			switch metricType {
			case "memory_usage":
				metric.Unit = "MB"
			case "cpu_usage":
				metric.Unit = "percent"
			case "execution_time":
				metric.Unit = "ms"
			case "throughput":
				metric.Unit = "ops/sec"
			default:
				metric.Unit = "unit"
			}
			
			if err := tx.Create(metric).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetExecutionMetrics retrieves metrics for an execution within a time range
func (r *ExecutionRepository) GetExecutionMetrics(ctx context.Context, executionID string, metricType string, start, end time.Time) ([]*ExecutionMetric, error) {
	query := r.db.WithContext(ctx).
		Where("execution_id = ?", executionID).
		Where("timestamp BETWEEN ? AND ?", start, end)
	
	if metricType != "" {
		query = query.Where("metric_type = ?", metricType)
	}
	
	var metrics []*ExecutionMetric
	err := query.Order("timestamp ASC").Find(&metrics).Error
	
	return metrics, err
}

// GetAggregatedMetrics returns aggregated metrics over time intervals
func (r *ExecutionRepository) GetAggregatedMetrics(ctx context.Context, workflowID string, metricType string, interval time.Duration, start, end time.Time) ([]*TimeSeriesData, error) {
	// Determine the appropriate time bucket based on interval
	var timeBucket string
	switch {
	case interval < time.Hour:
		timeBucket = "date_trunc('minute', em.timestamp)"
	case interval < 24*time.Hour:
		timeBucket = "date_trunc('hour', em.timestamp)"
	default:
		timeBucket = "date_trunc('day', em.timestamp)"
	}
	
	query := fmt.Sprintf(`
		SELECT 
			%s as timestamp,
			COUNT(*) as count,
			AVG(em.value) as value
		FROM execution_metrics em
		JOIN workflow_executions we ON we.id = em.execution_id
		WHERE we.workflow_id = ?
			AND em.metric_type = ?
			AND em.timestamp BETWEEN ? AND ?
		GROUP BY timestamp
		ORDER BY timestamp ASC
	`, timeBucket)
	
	var results []*TimeSeriesData
	err := r.db.WithContext(ctx).Raw(query, workflowID, metricType, start, end).Scan(&results).Error
	
	return results, err
}

// GetExecutionTimeline returns a timeline of execution events
func (r *ExecutionRepository) GetExecutionTimeline(ctx context.Context, executionID string) ([]*ExecutionEvent, error) {
	var events []*ExecutionEvent
	
	// Get state transitions
	transitions, err := r.GetStateTransitions(ctx, executionID)
	if err != nil {
		return nil, err
	}
	
	for _, t := range transitions {
		events = append(events, &ExecutionEvent{
			ID:        t.ID,
			Type:      "state_change",
			Timestamp: t.Timestamp,
			Data: map[string]interface{}{
				"from":     t.FromState,
				"to":       t.ToState,
				"metadata": t.Metadata,
			},
		})
	}
	
	// Get node executions
	nodeExecs, err := r.GetNodeExecutions(ctx, executionID)
	if err != nil {
		return nil, err
	}
	
	for _, ne := range nodeExecs {
		if !ne.StartedAt.IsZero() {
			events = append(events, &ExecutionEvent{
				ID:        ne.ID,
				Type:      "node_started",
				Timestamp: ne.StartedAt,
				Data: map[string]interface{}{
					"nodeId": ne.NodeID,
					"status": ne.Status,
				},
			})
		}
		
		if ne.FinishedAt != nil {
			events = append(events, &ExecutionEvent{
				ID:        ne.ID,
				Type:      "node_finished",
				Timestamp: *ne.FinishedAt,
				Data: map[string]interface{}{
					"nodeId": ne.NodeID,
					"status": ne.Status,
					"error":  ne.Error,
				},
			})
		}
	}
	
	// Sort events by timestamp
	sortExecutionEvents(events)
	
	return events, nil
}

// GetPerformanceHistory retrieves historical performance data
func (r *ExecutionRepository) GetPerformanceHistory(ctx context.Context, workflowID string, days int) (*PerformanceHistory, error) {
	since := time.Now().AddDate(0, 0, -days)
	
	history := &PerformanceHistory{
		WorkflowID: workflowID,
		Period:     fmt.Sprintf("Last %d days", days),
	}
	
	// Get daily execution counts
	query := `
		SELECT 
			date_trunc('day', started_at) as date,
			COUNT(*) as total,
			COUNT(CASE WHEN status = ? THEN 1 END) as successful,
			COUNT(CASE WHEN status = ? THEN 1 END) as failed,
			AVG(execution_time) as avg_time,
			MIN(execution_time) as min_time,
			MAX(execution_time) as max_time,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY execution_time) as median_time,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY execution_time) as p95_time
		FROM workflow_executions
		WHERE workflow_id = ?
			AND started_at >= ?
		GROUP BY date
		ORDER BY date ASC
	`
	
	type DailyStats struct {
		Date       time.Time
		Total      int64
		Successful int64
		Failed     int64
		AvgTime    float64
		MinTime    float64
		MaxTime    float64
		MedianTime float64
		P95Time    float64
	}
	
	var dailyStats []DailyStats
	err := r.db.WithContext(ctx).Raw(query, 
		workflow.ExecutionCompleted, 
		workflow.ExecutionFailed,
		workflowID, 
		since).Scan(&dailyStats).Error
	
	if err != nil {
		return nil, err
	}
	
	// Convert to performance history format
	for _, stat := range dailyStats {
		history.DailyExecutions = append(history.DailyExecutions, stat.Total)
		history.DailySuccessRate = append(history.DailySuccessRate, 
			float64(stat.Successful)/float64(stat.Total)*100)
		history.DailyAvgTime = append(history.DailyAvgTime, stat.AvgTime)
		history.DailyP95Time = append(history.DailyP95Time, stat.P95Time)
		history.Dates = append(history.Dates, stat.Date)
	}
	
	// Calculate overall statistics
	if len(dailyStats) > 0 {
		var totalExecs, totalSuccess int64
		var totalTime float64
		
		for _, stat := range dailyStats {
			totalExecs += stat.Total
			totalSuccess += stat.Successful
			totalTime += stat.AvgTime * float64(stat.Total)
		}
		
		history.TotalExecutions = totalExecs
		history.OverallSuccessRate = float64(totalSuccess) / float64(totalExecs) * 100
		history.OverallAvgTime = totalTime / float64(totalExecs)
	}
	
	return history, nil
}

// ArchiveOldMetrics archives metrics older than specified days
func (r *ExecutionRepository) ArchiveOldMetrics(ctx context.Context, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
	
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create archive entries
		archiveQuery := `
			INSERT INTO execution_metrics_archive (
				execution_id, node_id, metric_type, 
				avg_value, min_value, max_value, count,
				date, created_at
			)
			SELECT 
				execution_id,
				node_id,
				metric_type,
				AVG(value) as avg_value,
				MIN(value) as min_value,
				MAX(value) as max_value,
				COUNT(*) as count,
				date_trunc('day', timestamp) as date,
				NOW() as created_at
			FROM execution_metrics
			WHERE timestamp < ?
			GROUP BY execution_id, node_id, metric_type, date
		`
		
		if err := tx.Exec(archiveQuery, cutoffDate).Error; err != nil {
			return fmt.Errorf("failed to archive metrics: %w", err)
		}
		
		// Delete archived metrics
		if err := tx.Where("timestamp < ?", cutoffDate).
			Delete(&ExecutionMetric{}).Error; err != nil {
			return fmt.Errorf("failed to delete old metrics: %w", err)
		}
		
		// Delete old state transitions
		if err := tx.Where("timestamp < ?", cutoffDate).
			Delete(&StateTransition{}).Error; err != nil {
			return fmt.Errorf("failed to delete old transitions: %w", err)
		}
		
		return nil
	})
}

// Types for execution timeline and performance history

type ExecutionEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

type PerformanceHistory struct {
	WorkflowID         string      `json:"workflowId"`
	Period             string      `json:"period"`
	Dates              []time.Time `json:"dates"`
	DailyExecutions    []int64     `json:"dailyExecutions"`
	DailySuccessRate   []float64   `json:"dailySuccessRate"`
	DailyAvgTime       []float64   `json:"dailyAvgTime"`
	DailyP95Time       []float64   `json:"dailyP95Time"`
	TotalExecutions    int64       `json:"totalExecutions"`
	OverallSuccessRate float64     `json:"overallSuccessRate"`
	OverallAvgTime     float64     `json:"overallAvgTime"`
}

// Helper function to sort execution events by timestamp
func sortExecutionEvents(events []*ExecutionEvent) {
	for i := 0; i < len(events)-1; i++ {
		for j := i + 1; j < len(events); j++ {
			if events[i].Timestamp.After(events[j].Timestamp) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}
}

// GetRealtimeMetrics returns real-time metrics for active executions
func (r *ExecutionRepository) GetRealtimeMetrics(ctx context.Context, workflowID string) (*RealtimeMetrics, error) {
	metrics := &RealtimeMetrics{
		WorkflowID: workflowID,
		Timestamp:  time.Now(),
	}
	
	// Get running executions
	var runningCount int64
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ? AND status = ?", workflowID, workflow.ExecutionRunning).
		Count(&runningCount)
	metrics.RunningExecutions = int(runningCount)
	
	// Get recent completion rate (last hour)
	lastHour := time.Now().Add(-time.Hour)
	
	var recentStats struct {
		Total      int64
		Successful int64
		Failed     int64
		AvgTime    float64
	}
	
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Select(`
			COUNT(*) as total,
			COUNT(CASE WHEN status = ? THEN 1 END) as successful,
			COUNT(CASE WHEN status = ? THEN 1 END) as failed,
			AVG(execution_time) as avg_time
		`, workflow.ExecutionCompleted, workflow.ExecutionFailed).
		Where("workflow_id = ? AND finished_at >= ?", workflowID, lastHour).
		Scan(&recentStats)
	
	metrics.RecentCompletions = int(recentStats.Total)
	if recentStats.Total > 0 {
		metrics.RecentSuccessRate = float64(recentStats.Successful) / float64(recentStats.Total) * 100
	}
	metrics.RecentAvgTime = recentStats.AvgTime
	
	// Get current throughput (executions per minute in last 5 minutes)
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	var throughputCount int64
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ? AND started_at >= ?", workflowID, fiveMinutesAgo).
		Count(&throughputCount)
	metrics.CurrentThroughput = float64(throughputCount) / 5.0
	
	return metrics, nil
}

type RealtimeMetrics struct {
	WorkflowID        string    `json:"workflowId"`
	Timestamp         time.Time `json:"timestamp"`
	RunningExecutions int       `json:"runningExecutions"`
	RecentCompletions int       `json:"recentCompletions"`
	RecentSuccessRate float64   `json:"recentSuccessRate"`
	RecentAvgTime     float64   `json:"recentAvgTime"`
	CurrentThroughput float64   `json:"currentThroughput"`
}
