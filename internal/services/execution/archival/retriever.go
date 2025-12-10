package archival

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/database"
	"gorm.io/gorm"
)

// Retriever handles retrieval of archived data
type Retriever struct {
	db         *database.DB
	storage    Storage
	compressor Compressor
}

// NewRetriever creates a new retriever
func NewRetriever(db *database.DB, storage Storage, compressor Compressor) *Retriever {
	return &Retriever{
		db:         db,
		storage:    storage,
		compressor: compressor,
	}
}

// RetrieveExecution retrieves an archived execution
func (r *Retriever) RetrieveExecution(ctx context.Context, executionID string) (*workflow.WorkflowExecution, error) {
	// First check if execution exists in hot storage
	var execution workflow.WorkflowExecution
	err := r.db.WithContext(ctx).
		Preload("NodeExecutions").
		Where("id = ?", executionID).
		First(&execution).Error
	
	if err == nil {
		return &execution, nil
	}
	
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	
	// Search in archives
	return r.retrieveFromArchive(ctx, executionID)
}

// retrieveFromArchive searches for execution in archives
func (r *Retriever) retrieveFromArchive(ctx context.Context, executionID string) (*workflow.WorkflowExecution, error) {
	// Query archive metadata
	var metadata []ArchiveMetadata
	err := r.db.WithContext(ctx).
		Where("type = ?", "executions").
		Order("created_at DESC").
		Find(&metadata).Error
	
	if err != nil {
		return nil, err
	}
	
	// Search through archives
	for _, meta := range metadata {
		// Download archive
		compressed, err := r.storage.Download(ctx, meta.StorageKey)
		if err != nil {
			continue // Skip if download fails
		}
		
		// Decompress
		data, err := r.compressor.Decompress(compressed)
		if err != nil {
			continue
		}
		
		// Deserialize
		var archive ExecutionArchive
		if err := json.Unmarshal(data, &archive); err != nil {
			continue
		}
		
		// Search for execution
		for _, exec := range archive.Executions {
			if exec.ID == executionID {
				return &exec, nil
			}
		}
	}
	
	return nil, fmt.Errorf("execution not found in archives")
}

// QueryArchivedExecutions queries archived executions
func (r *Retriever) QueryArchivedExecutions(ctx context.Context, filter ArchivedExecutionFilter) ([]*workflow.WorkflowExecution, error) {
	// Determine date range
	startDate := filter.StartDate.Format("2006-01-02")
	endDate := filter.EndDate.Format("2006-01-02")
	
	// Query relevant archives
	var metadata []ArchiveMetadata
	query := r.db.WithContext(ctx).
		Where("type = ?", "executions").
		Where("date >= ? AND date <= ?", startDate, endDate)
	
	if err := query.Find(&metadata).Error; err != nil {
		return nil, err
	}
	
	var allExecutions []*workflow.WorkflowExecution
	
	// Process each archive
	for _, meta := range metadata {
		executions, err := r.loadArchive(ctx, meta.StorageKey)
		if err != nil {
			continue
		}
		
		// Filter executions
		for _, exec := range executions {
			if r.matchesFilter(exec, filter) {
				allExecutions = append(allExecutions, &exec)
			}
		}
	}
	
	// Apply limit if specified
	if filter.Limit > 0 && len(allExecutions) > filter.Limit {
		allExecutions = allExecutions[:filter.Limit]
	}
	
	return allExecutions, nil
}

// loadArchive loads executions from an archive
func (r *Retriever) loadArchive(ctx context.Context, storageKey string) ([]workflow.WorkflowExecution, error) {
	// Download from storage
	compressed, err := r.storage.Download(ctx, storageKey)
	if err != nil {
		return nil, err
	}
	
	// Decompress
	data, err := r.compressor.Decompress(compressed)
	if err != nil {
		return nil, err
	}
	
	// Deserialize
	var archive ExecutionArchive
	if err := json.Unmarshal(data, &archive); err != nil {
		return nil, err
	}
	
	return archive.Executions, nil
}

// matchesFilter checks if execution matches filter criteria
func (r *Retriever) matchesFilter(exec workflow.WorkflowExecution, filter ArchivedExecutionFilter) bool {
	// Check workflow ID
	if filter.WorkflowID != "" && exec.WorkflowID != filter.WorkflowID {
		return false
	}
	
	// Check status
	if filter.Status != "" && exec.Status != filter.Status {
		return false
	}
	
	// Check user ID
	if filter.UserID != "" && exec.CreatedBy != filter.UserID {
		return false
	}
	
	// Check date range
	if !filter.StartDate.IsZero() && exec.CreatedAt.Before(filter.StartDate) {
		return false
	}
	
	if !filter.EndDate.IsZero() && exec.CreatedAt.After(filter.EndDate) {
		return false
	}
	
	return true
}

// GetUnifiedExecutions retrieves executions from both hot and cold storage
func (r *Retriever) GetUnifiedExecutions(ctx context.Context, filter UnifiedExecutionFilter) ([]*workflow.WorkflowExecution, error) {
	var allExecutions []*workflow.WorkflowExecution
	
	// Get from hot storage
	if filter.IncludeHotStorage {
		var hotExecutions []*workflow.WorkflowExecution
		
		query := r.db.WithContext(ctx).Model(&workflow.WorkflowExecution{})
		
		if filter.WorkflowID != "" {
			query = query.Where("workflow_id = ?", filter.WorkflowID)
		}
		
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		
		if !filter.StartDate.IsZero() {
			query = query.Where("created_at >= ?", filter.StartDate)
		}
		
		if !filter.EndDate.IsZero() {
			query = query.Where("created_at <= ?", filter.EndDate)
		}
		
		if err := query.Find(&hotExecutions).Error; err != nil {
			return nil, err
		}
		
		allExecutions = append(allExecutions, hotExecutions...)
	}
	
	// Get from cold storage
	if filter.IncludeColdStorage {
		archivedFilter := ArchivedExecutionFilter{
			WorkflowID: filter.WorkflowID,
			Status:     filter.Status,
			UserID:     filter.UserID,
			StartDate:  filter.StartDate,
			EndDate:    filter.EndDate,
			Limit:      filter.Limit,
		}
		
		archivedExecutions, err := r.QueryArchivedExecutions(ctx, archivedFilter)
		if err != nil {
			return nil, err
		}
		
		allExecutions = append(allExecutions, archivedExecutions...)
	}
	
	// Sort by created date (newest first)
	sortExecutionsByDate(allExecutions)
	
	// Apply limit
	if filter.Limit > 0 && len(allExecutions) > filter.Limit {
		allExecutions = allExecutions[:filter.Limit]
	}
	
	return allExecutions, nil
}

// RestoreToHotStorage restores an archived execution back to hot storage
func (r *Retriever) RestoreToHotStorage(ctx context.Context, executionID string) error {
	// Retrieve from archive
	execution, err := r.retrieveFromArchive(ctx, executionID)
	if err != nil {
		return err
	}
	
	// Save back to database
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create workflow execution
		if err := tx.Create(execution).Error; err != nil {
			return err
		}
		
		// Create node executions
		for _, nodeExec := range execution.NodeExecutions {
			if err := tx.Create(&nodeExec).Error; err != nil {
				return err
			}
		}
		
		return nil
	})
}

// Filter types

// ArchivedExecutionFilter filters for archived executions
type ArchivedExecutionFilter struct {
	WorkflowID string
	Status     string
	UserID     string
	StartDate  time.Time
	EndDate    time.Time
	Limit      int
}

// UnifiedExecutionFilter filters for both hot and cold storage
type UnifiedExecutionFilter struct {
	WorkflowID         string
	Status             string
	UserID             string
	StartDate          time.Time
	EndDate            time.Time
	Limit              int
	IncludeHotStorage  bool
	IncludeColdStorage bool
}

// Helper function to sort executions by date
func sortExecutionsByDate(executions []*workflow.WorkflowExecution) {
	for i := 0; i < len(executions)-1; i++ {
		for j := i + 1; j < len(executions); j++ {
			if executions[i].CreatedAt.Before(executions[j].CreatedAt) {
				executions[i], executions[j] = executions[j], executions[i]
			}
		}
	}
}
