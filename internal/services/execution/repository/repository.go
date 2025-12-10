package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ExecutionRepository struct {
	db *database.DB
}

func NewExecutionRepository(db *database.DB) *ExecutionRepository {
	return &ExecutionRepository{db: db}
}

func (r *ExecutionRepository) Create(ctx context.Context, execution *workflow.WorkflowExecution) error {
	execution.CreatedAt = time.Now()
	
	// Record initial state transition
	transition := &StateTransition{
		ID:          uuid.New().String(),
		ExecutionID: execution.ID,
		FromState:   "",
		ToState:     execution.Status,
		Timestamp:   time.Now(),
		Metadata:    map[string]interface{}{"action": "created"},
	}
	
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(execution).Error; err != nil {
			return err
		}
		return tx.Create(transition).Error
	})
}

func (r *ExecutionRepository) Update(ctx context.Context, execution *workflow.WorkflowExecution) error {
	// Calculate execution time if finished
	if execution.FinishedAt != nil && !execution.StartedAt.IsZero() {
		execution.ExecutionTime = int64(execution.FinishedAt.Sub(execution.StartedAt).Milliseconds())
	}
	
	return r.db.WithContext(ctx).Save(execution).Error
}

// UpdateState updates execution state with atomic state transition recording
func (r *ExecutionRepository) UpdateState(ctx context.Context, id string, newState string, metadata map[string]interface{}) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get current state with row lock
		var execution workflow.WorkflowExecution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).
			First(&execution).Error; err != nil {
			return err
		}
		
		oldState := execution.Status
		
		// Update execution state
		updates := map[string]interface{}{
			"status": newState,
		}
		
		// Set timestamps based on state
		switch newState {
		case workflow.ExecutionRunning:
			if execution.StartedAt.IsZero() {
				now := time.Now()
				updates["started_at"] = now
			}
		case workflow.ExecutionCompleted, workflow.ExecutionFailed, workflow.ExecutionCancelled:
			if execution.FinishedAt == nil {
				now := time.Now()
				updates["finished_at"] = &now
				if !execution.StartedAt.IsZero() {
					updates["execution_time"] = int64(now.Sub(execution.StartedAt).Milliseconds())
				}
			}
		}
		
		if err := tx.Model(&workflow.WorkflowExecution{}).
			Where("id = ?", id).
			Updates(updates).Error; err != nil {
			return err
		}
		
		// Record state transition
		transition := &StateTransition{
			ID:          uuid.New().String(),
			ExecutionID: id,
			FromState:   oldState,
			ToState:     newState,
			Timestamp:   time.Now(),
			Metadata:    metadata,
		}
		
		return tx.Create(transition).Error
	})
}

func (r *ExecutionRepository) GetByID(ctx context.Context, id string) (*workflow.WorkflowExecution, error) {
	var execution workflow.WorkflowExecution
	err := r.db.WithContext(ctx).
		Preload("NodeExecutions").
		Where("id = ?", id).
		First(&execution).Error
	
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("execution not found")
	}
	
	return &execution, err
}

func (r *ExecutionRepository) GetWorkflow(ctx context.Context, workflowID string) (*workflow.Workflow, error) {
	var wf workflow.Workflow
	err := r.db.WithContext(ctx).
		Where("id = ?", workflowID).
		First(&wf).Error
	
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("workflow not found")
	}
	
	return &wf, err
}

func (r *ExecutionRepository) CreateNodeExecution(ctx context.Context, nodeExec *workflow.NodeExecution) error {
	return r.db.WithContext(ctx).Create(nodeExec).Error
}

func (r *ExecutionRepository) UpdateNodeExecution(ctx context.Context, nodeExec *workflow.NodeExecution) error {
	return r.db.WithContext(ctx).Save(nodeExec).Error
}

func (r *ExecutionRepository) GetNodeExecutions(ctx context.Context, executionID string) ([]*workflow.NodeExecution, error) {
	var nodeExecutions []*workflow.NodeExecution
	err := r.db.WithContext(ctx).
		Where("execution_id = ?", executionID).
		Order("started_at ASC").
		Find(&nodeExecutions).Error
	
	return nodeExecutions, err
}

func (r *ExecutionRepository) ListExecutions(ctx context.Context, filter ExecutionFilter, pagination *database.Pagination) ([]*workflow.WorkflowExecution, error) {
	query := r.db.WithContext(ctx).Model(&workflow.WorkflowExecution{})
	
	// Apply filters
	if filter.WorkflowID != "" {
		query = query.Where("workflow_id = ?", filter.WorkflowID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.UserID != "" {
		query = query.Where("created_by = ?", filter.UserID)
	}
	if !filter.StartedAfter.IsZero() {
		query = query.Where("started_at >= ?", filter.StartedAfter)
	}
	if !filter.StartedBefore.IsZero() {
		query = query.Where("started_at <= ?", filter.StartedBefore)
	}
	
	var executions []*workflow.WorkflowExecution
	err := r.db.Paginate(ctx, &executions, pagination, query)
	
	return executions, err
}

func (r *ExecutionRepository) GetRunningExecutions(ctx context.Context) ([]*workflow.WorkflowExecution, error) {
	var executions []*workflow.WorkflowExecution
	err := r.db.WithContext(ctx).
		Where("status = ?", workflow.ExecutionRunning).
		Find(&executions).Error
	
	return executions, err
}

func (r *ExecutionRepository) GetExecutionStats(ctx context.Context, workflowID string) (*ExecutionStats, error) {
	var stats ExecutionStats
	
	// Total executions
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ?", workflowID).
		Count(&stats.Total)
	
	// Successful executions
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ? AND status = ?", workflowID, workflow.ExecutionCompleted).
		Count(&stats.Successful)
	
	// Failed executions
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ? AND status = ?", workflowID, workflow.ExecutionFailed).
		Count(&stats.Failed)
	
	// Running executions
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ? AND status = ?", workflowID, workflow.ExecutionRunning).
		Count(&stats.Running)
	
	// Average execution time
	var avgTime float64
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ? AND execution_time > 0", workflowID).
		Select("AVG(execution_time)").
		Scan(&avgTime)
	stats.AverageExecutionTime = avgTime
	
	// Last execution
	var lastExecution workflow.WorkflowExecution
	if err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("started_at DESC").
		First(&lastExecution).Error; err == nil {
		stats.LastExecutionAt = &lastExecution.StartedAt
	}
	
	return &stats, nil
}

func (r *ExecutionRepository) GetGlobalStats(ctx context.Context) (*GlobalExecutionStats, error) {
	var stats GlobalExecutionStats
	
	// Total executions today
	today := time.Now().Truncate(24 * time.Hour)
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("started_at >= ?", today).
		Count(&stats.ExecutionsToday)
	
	// Total executions this week
	weekAgo := time.Now().AddDate(0, 0, -7)
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("started_at >= ?", weekAgo).
		Count(&stats.ExecutionsThisWeek)
	
	// Total executions this month
	monthAgo := time.Now().AddDate(0, -1, 0)
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("started_at >= ?", monthAgo).
		Count(&stats.ExecutionsThisMonth)
	
	// Success rate
	var total, successful int64
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("started_at >= ?", monthAgo).
		Count(&total)
	
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("started_at >= ? AND status = ?", monthAgo, workflow.ExecutionCompleted).
		Count(&successful)
	
	if total > 0 {
		stats.SuccessRate = float64(successful) / float64(total) * 100
	}
	
	// Most executed workflows
	type WorkflowCount struct {
		WorkflowID string
		Count      int64
	}
	
	var workflowCounts []WorkflowCount
	r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Select("workflow_id, COUNT(*) as count").
		Where("started_at >= ?", monthAgo).
		Group("workflow_id").
		Order("count DESC").
		Limit(10).
		Scan(&workflowCounts)
	
	stats.TopWorkflows = make([]string, len(workflowCounts))
	for i, wc := range workflowCounts {
		stats.TopWorkflows[i] = wc.WorkflowID
	}
	
	return &stats, nil
}

func (r *ExecutionRepository) CleanupOldExecutions(ctx context.Context, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
	
	// Delete old node executions first
	if err := r.db.WithContext(ctx).
		Where("created_at < ?", cutoffDate).
		Delete(&workflow.NodeExecution{}).Error; err != nil {
		return fmt.Errorf("failed to delete old node executions: %w", err)
	}
	
	// Delete old workflow executions
	if err := r.db.WithContext(ctx).
		Where("created_at < ?", cutoffDate).
		Delete(&workflow.WorkflowExecution{}).Error; err != nil {
		return fmt.Errorf("failed to delete old workflow executions: %w", err)
	}
	
	return nil
}

// Filter and stats types
type ExecutionFilter struct {
	WorkflowID    string
	Status        string
	UserID        string
	StartedAfter  time.Time
	StartedBefore time.Time
}

type ExecutionStats struct {
	Total                int64
	Successful           int64
	Failed               int64
	Running              int64
	AverageExecutionTime float64
	LastExecutionAt      *time.Time
}

type GlobalExecutionStats struct {
	ExecutionsToday     int64
	ExecutionsThisWeek  int64
	ExecutionsThisMonth int64
	SuccessRate         float64
	TopWorkflows        []string
}
