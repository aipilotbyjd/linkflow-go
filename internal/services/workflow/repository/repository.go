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

type WorkflowRepository struct {
	db *database.DB
}

func NewWorkflowRepository(db *database.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

// CreateWithVersion creates a new workflow with initial version
func (r *WorkflowRepository) CreateWithVersion(ctx context.Context, w *workflow.Workflow) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create workflow
		if err := tx.Create(w).Error; err != nil {
			return err
		}
		
		// Create initial version
		workflowJSON, err := w.ToJSON()
		if err != nil {
			return err
		}
		
		version := &workflow.WorkflowVersion{
			ID:         uuid.New().String(),
			WorkflowID: w.ID,
			Version:    1,
			Data:       workflowJSON,
			ChangedBy:  w.UserID,
			ChangeNote: "Initial version",
			CreatedAt:  time.Now(),
		}
		
		return tx.Create(version).Error
	})
}

// GetWithNodes retrieves a workflow with all its nodes and connections
func (r *WorkflowRepository) GetWithNodes(ctx context.Context, id string) (*workflow.Workflow, error) {
	var w workflow.Workflow
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		First(&w).Error
	
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("workflow not found")
	}
	
	return &w, err
}

// GetByIDAndUser retrieves a workflow by ID and user ID
func (r *WorkflowRepository) GetByIDAndUser(ctx context.Context, workflowID, userID string) (*workflow.Workflow, error) {
	var w workflow.Workflow
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", workflowID, userID).
		Where("deleted_at IS NULL").
		First(&w).Error
	
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("workflow not found")
	}
	
	return &w, err
}

// GetByIDAndTeam retrieves a workflow by ID and team ID
func (r *WorkflowRepository) GetByIDAndTeam(ctx context.Context, workflowID, teamID string) (*workflow.Workflow, error) {
	var w workflow.Workflow
	err := r.db.WithContext(ctx).
		Where("id = ? AND team_id = ?", workflowID, teamID).
		Where("deleted_at IS NULL").
		First(&w).Error
	
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("workflow not found")
	}
	
	return &w, err
}

// UpdateWithVersion updates a workflow and creates a new version
func (r *WorkflowRepository) UpdateWithVersion(ctx context.Context, w *workflow.Workflow, changeNote string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get current version
		var currentVersion int
		err := tx.Model(&workflow.WorkflowVersion{}).
			Where("workflow_id = ?", w.ID).
			Select("MAX(version)").
			Scan(&currentVersion).Error
		if err != nil {
			return err
		}
		
		// Increment version
		w.Version = currentVersion + 1
		w.UpdatedAt = time.Now()
		
		// Update workflow
		if err := tx.Save(w).Error; err != nil {
			return err
		}
		
		// Create new version
		workflowJSON, err := w.ToJSON()
		if err != nil {
			return err
		}
		
		version := &workflow.WorkflowVersion{
			ID:         uuid.New().String(),
			WorkflowID: w.ID,
			Version:    w.Version,
			Data:       workflowJSON,
			ChangedBy:  w.UserID,
			ChangeNote: changeNote,
			CreatedAt:  time.Now(),
		}
		
		return tx.Create(version).Error
	})
}

// GetVersion retrieves a specific version of a workflow
func (r *WorkflowRepository) GetVersion(ctx context.Context, workflowID string, version int) (*workflow.WorkflowVersion, error) {
	var wv workflow.WorkflowVersion
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND version = ?", workflowID, version).
		First(&wv).Error
	
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("workflow version not found")
	}
	
	return &wv, err
}

// ListVersions lists all versions of a workflow
func (r *WorkflowRepository) ListVersions(ctx context.Context, workflowID string) ([]*workflow.WorkflowVersion, error) {
	var versions []*workflow.WorkflowVersion
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("version DESC").
		Find(&versions).Error
	
	return versions, err
}

// RestoreVersion restores a workflow to a specific version
func (r *WorkflowRepository) RestoreVersion(ctx context.Context, workflowID string, version int, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get the version to restore
		var wv workflow.WorkflowVersion
		if err := tx.Where("workflow_id = ? AND version = ?", workflowID, version).First(&wv).Error; err != nil {
			return err
		}
		
		// Parse the workflow data
		var restoredWorkflow workflow.Workflow
		if err := json.Unmarshal([]byte(wv.Data), &restoredWorkflow); err != nil {
			return err
		}
		
		// Get current version number
		var currentVersion int
		err := tx.Model(&workflow.WorkflowVersion{}).
			Where("workflow_id = ?", workflowID).
			Select("MAX(version)").
			Scan(&currentVersion).Error
		if err != nil {
			return err
		}
		
		// Update workflow with restored data
		restoredWorkflow.Version = currentVersion + 1
		restoredWorkflow.UpdatedAt = time.Now()
		
		if err := tx.Save(&restoredWorkflow).Error; err != nil {
			return err
		}
		
		// Create new version record
		newVersion := &workflow.WorkflowVersion{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			Version:    restoredWorkflow.Version,
			Data:       wv.Data,
			ChangedBy:  userID,
			ChangeNote: fmt.Sprintf("Restored from version %d", version),
			CreatedAt:  time.Now(),
		}
		
		return tx.Create(newVersion).Error
	})
}

// ListWorkflows lists workflows with filters and pagination
func (r *WorkflowRepository) ListWorkflows(ctx context.Context, opts ListWorkflowsOptions) ([]*workflow.Workflow, int64, error) {
	var workflows []*workflow.Workflow
	var total int64
	
	query := r.db.WithContext(ctx).Model(&workflow.Workflow{})
	
	// Apply filters
	if opts.UserID != "" {
		query = query.Where("user_id = ?", opts.UserID)
	}
	
	if opts.TeamID != "" {
		query = query.Where("team_id = ?", opts.TeamID)
	}
	
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	
	if opts.IsActive != nil {
		query = query.Where("is_active = ?", *opts.IsActive)
	}
	
	// Filter by tags
	if len(opts.Tags) > 0 {
		query = query.Where("tags && ?", opts.Tags)
	}
	
	// Search by name or description
	if opts.Search != "" {
		searchTerm := "%" + opts.Search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchTerm, searchTerm)
	}
	
	// Exclude deleted
	query = query.Where("deleted_at IS NULL")
	
	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Apply sorting
	if opts.SortBy != "" {
		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: opts.SortBy}, Desc: opts.SortDesc})
	} else {
		query = query.Order("updated_at DESC")
	}
	
	// Apply pagination
	if opts.Page > 0 && opts.Limit > 0 {
		offset := (opts.Page - 1) * opts.Limit
		query = query.Offset(offset).Limit(opts.Limit)
	}
	
	err := query.Find(&workflows).Error
	return workflows, total, err
}

// Clone creates a copy of a workflow
func (r *WorkflowRepository) Clone(ctx context.Context, workflowID, userID, newName string) (*workflow.Workflow, error) {
	var original workflow.Workflow
	
	// Get original workflow
	if err := r.db.WithContext(ctx).Where("id = ?", workflowID).First(&original).Error; err != nil {
		return nil, err
	}
	
	// Create clone
	clone := original.Clone(newName)
	clone.UserID = userID
	
	// Save clone with initial version
	if err := r.CreateWithVersion(ctx, clone); err != nil {
		return nil, err
	}
	
	return clone, nil
}

// BulkUpdateStatus updates status for multiple workflows
func (r *WorkflowRepository) BulkUpdateStatus(ctx context.Context, workflowIDs []string, status string) error {
	return r.db.WithContext(ctx).
		Model(&workflow.Workflow{}).
		Where("id IN ?", workflowIDs).
		Updates(map[string]interface{}{
			"status":     status,
			"is_active":  status == workflow.StatusActive,
			"updated_at": time.Now(),
		}).Error
}

// GetActiveWorkflows retrieves all active workflows
func (r *WorkflowRepository) GetActiveWorkflows(ctx context.Context) ([]*workflow.Workflow, error) {
	var workflows []*workflow.Workflow
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND deleted_at IS NULL", true).
		Find(&workflows).Error
	
	return workflows, err
}

// GetWorkflowsByNodeType retrieves workflows containing specific node type
func (r *WorkflowRepository) GetWorkflowsByNodeType(ctx context.Context, nodeType string) ([]*workflow.Workflow, error) {
	var workflows []*workflow.Workflow
	
	query := fmt.Sprintf(`
		SELECT * FROM workflows 
		WHERE deleted_at IS NULL 
		AND EXISTS (
			SELECT 1 FROM jsonb_array_elements(nodes) AS node 
			WHERE node->>'type' = '%s'
		)
	`, nodeType)
	
	err := r.db.WithContext(ctx).Raw(query).Scan(&workflows).Error
	return workflows, err
}

// CountWorkflowsByStatus returns count of workflows grouped by status
func (r *WorkflowRepository) CountWorkflowsByStatus(ctx context.Context, userID string) (map[string]int64, error) {
	type StatusCount struct {
		Status string
		Count  int64
	}
	
	var counts []StatusCount
	result := make(map[string]int64)
	
	query := r.db.WithContext(ctx).
		Model(&workflow.Workflow{}).
		Select("status, COUNT(*) as count").
		Where("deleted_at IS NULL")
	
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	
	err := query.Group("status").Scan(&counts).Error
	if err != nil {
		return nil, err
	}
	
	for _, sc := range counts {
		result[sc.Status] = sc.Count
	}
	
	return result, nil
}

// GetRecentlyModified retrieves recently modified workflows
func (r *WorkflowRepository) GetRecentlyModified(ctx context.Context, userID string, limit int) ([]*workflow.Workflow, error) {
	var workflows []*workflow.Workflow
	
	query := r.db.WithContext(ctx).
		Where("deleted_at IS NULL")
	
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	
	err := query.
		Order("updated_at DESC").
		Limit(limit).
		Find(&workflows).Error
	
	return workflows, err
}

// UpdateNodes updates only the nodes of a workflow
func (r *WorkflowRepository) UpdateNodes(ctx context.Context, workflowID string, nodes []workflow.Node) error {
	nodesJSON, err := json.Marshal(nodes)
	if err != nil {
		return err
	}
	
	return r.db.WithContext(ctx).
		Model(&workflow.Workflow{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"nodes":      nodesJSON,
			"updated_at": time.Now(),
		}).Error
}

// UpdateConnections updates only the connections of a workflow
func (r *WorkflowRepository) UpdateConnections(ctx context.Context, workflowID string, connections []workflow.Connection) error {
	connectionsJSON, err := json.Marshal(connections)
	if err != nil {
		return err
	}
	
	return r.db.WithContext(ctx).
		Model(&workflow.Workflow{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"connections": connectionsJSON,
			"updated_at":  time.Now(),
		}).Error
}

// ListWorkflowsOptions represents options for listing workflows
type ListWorkflowsOptions struct {
	UserID   string
	TeamID   string
	Status   string
	IsActive *bool
	Tags     []string
	Search   string
	Page     int
	Limit    int
	SortBy   string
	SortDesc bool
}

// Deprecated methods for backward compatibility
func (r *WorkflowRepository) CreateWorkflow(ctx context.Context, w *workflow.Workflow) error {
	return r.CreateWithVersion(ctx, w)
}

func (r *WorkflowRepository) GetWorkflow(ctx context.Context, workflowID, userID string) (*workflow.Workflow, error) {
	return r.GetByIDAndUser(ctx, workflowID, userID)
}

func (r *WorkflowRepository) UpdateWorkflow(ctx context.Context, w *workflow.Workflow) error {
	return r.UpdateWithVersion(ctx, w, "Updated")
}

func (r *WorkflowRepository) DeleteWorkflow(ctx context.Context, workflowID, userID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&workflow.Workflow{}).
		Where("id = ? AND user_id = ?", workflowID, userID).
		Update("deleted_at", &now).Error
}
