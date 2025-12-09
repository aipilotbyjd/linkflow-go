package repository

import (
	"context"
	
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/database"
)

type WorkflowRepository struct {
	db *database.DB
}

func NewWorkflowRepository(db *database.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) CreateWorkflow(ctx context.Context, w *workflow.Workflow) error {
	return r.db.WithContext(ctx).Create(w).Error
}

func (r *WorkflowRepository) GetWorkflow(ctx context.Context, workflowID, userID string) (*workflow.Workflow, error) {
	var w workflow.Workflow
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", workflowID, userID).First(&w).Error
	return &w, err
}

func (r *WorkflowRepository) ListWorkflows(ctx context.Context, userID string, page, limit int, status string) ([]*workflow.Workflow, int64, error) {
	var workflows []*workflow.Workflow
	var total int64
	
	query := r.db.Model(&workflow.Workflow{}).Where("user_id = ?", userID)
	
	if status != "" {
		query = query.Where("status = ?", status)
	}
	
	query.Count(&total)
	
	err := query.
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&workflows).Error
		
	return workflows, total, err
}

func (r *WorkflowRepository) UpdateWorkflow(ctx context.Context, w *workflow.Workflow) error {
	return r.db.WithContext(ctx).Save(w).Error
}

func (r *WorkflowRepository) DeleteWorkflow(ctx context.Context, workflowID, userID string) error {
	return r.db.WithContext(ctx).Where("id = ? AND user_id = ?", workflowID, userID).Delete(&workflow.Workflow{}).Error
}
