package repository

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/database"
)

type SearchRepository struct {
	db *database.DB
}

func NewSearchRepository(db *database.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

// SearchWorkflows searches workflows by name and description
func (r *SearchRepository) SearchWorkflows(ctx context.Context, query string, userID string, limit, offset int) ([]*workflow.Workflow, int64, error) {
	var workflows []*workflow.Workflow
	var total int64

	baseQuery := r.db.WithContext(ctx).Model(&workflow.Workflow{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Where("name ILIKE ? OR description ILIKE ?", "%"+query+"%", "%"+query+"%")

	// Get total count
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if err := baseQuery.
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&workflows).Error; err != nil {
		return nil, 0, err
	}

	return workflows, total, nil
}

// SearchByTags searches workflows by tags
func (r *SearchRepository) SearchByTags(ctx context.Context, tags []string, userID string, limit, offset int) ([]*workflow.Workflow, int64, error) {
	var workflows []*workflow.Workflow
	var total int64

	baseQuery := r.db.WithContext(ctx).Model(&workflow.Workflow{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Where("tags ?| ?", tags)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&workflows).Error; err != nil {
		return nil, 0, err
	}

	return workflows, total, nil
}

// GetRecentWorkflows returns recently updated workflows
func (r *SearchRepository) GetRecentWorkflows(ctx context.Context, userID string, limit int) ([]*workflow.Workflow, error) {
	var workflows []*workflow.Workflow
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&workflows).Error
	return workflows, err
}
