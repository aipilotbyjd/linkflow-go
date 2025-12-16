package repository

import (
	"context"
	"time"

	"github.com/linkflow-go/internal/workflow/ports"
	"github.com/linkflow-go/pkg/contracts/workflow"
	"gorm.io/gorm"
)

func (r *WorkflowRepository) Ping(ctx context.Context) error {
	sqlDB, err := r.db.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.PingContext(ctx)
}

// Permissions

func (r *WorkflowRepository) ListWorkflowPermissions(ctx context.Context, workflowID string) ([]map[string]interface{}, error) {
	var permissions []map[string]interface{}
	err := r.db.WithContext(ctx).
		Table("workflow.workflow_permissions").
		Where("workflow_id = ?", workflowID).
		Find(&permissions).Error
	if err != nil {
		return nil, err
	}

	return permissions, nil
}

func (r *WorkflowRepository) CreateWorkflowPermission(ctx context.Context, permission map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Table("workflow.workflow_permissions").
		Create(&permission).Error
}

func (r *WorkflowRepository) DeleteWorkflowPermission(ctx context.Context, workflowID, userID string) (int64, error) {
	result := r.db.WithContext(ctx).
		Table("workflow.workflow_permissions").
		Where("workflow_id = ? AND user_id = ?", workflowID, userID).
		Delete(nil)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// Categories

func (r *WorkflowRepository) CreateCategory(ctx context.Context, category map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Table("workflow.categories").
		Create(&category).Error
}

// Stats & Executions

func (r *WorkflowRepository) GetWorkflowStats(ctx context.Context, workflowID string) (ports.WorkflowStats, error) {
	var stats ports.WorkflowStats

	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) as total_executions,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as successful_runs,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_runs,
			AVG(execution_time) as avg_execution_time,
			MAX(created_at) as last_execution_time
		FROM workflow.workflow_executions
		WHERE workflow_id = ?
	`, workflowID).Scan(&stats).Error

	return stats, err
}

func (r *WorkflowRepository) ListWorkflowExecutions(ctx context.Context, workflowID string, offset, limit int) ([]workflow.WorkflowExecution, int64, error) {
	var total int64
	var executions []workflow.WorkflowExecution

	if err := r.db.WithContext(ctx).
		Model(&workflow.WorkflowExecution{}).
		Where("workflow_id = ?", workflowID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&executions).Error
	if err != nil {
		return nil, 0, err
	}

	return executions, total, nil
}

func (r *WorkflowRepository) GetLatestWorkflowExecution(ctx context.Context, workflowID string) (*workflow.WorkflowExecution, error) {
	var exec workflow.WorkflowExecution
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("created_at DESC").
		First(&exec).Error
	if err != nil {
		return nil, err
	}

	return &exec, nil
}

func (r *WorkflowRepository) GetPopularTags(ctx context.Context, limit int) ([]string, error) {
	var tags []string

	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT unnest(tags) as tag
		FROM workflow.workflows
		WHERE deleted_at IS NULL
		GROUP BY tag
		ORDER BY COUNT(*) DESC
		LIMIT ?
	`, limit).Scan(&tags).Error
	if err != nil {
		return nil, err
	}

	return tags, nil
}

// Variables

func (r *WorkflowRepository) SaveWorkflowVariable(ctx context.Context, variable *workflow.WorkflowVariable) error {
	return r.db.WithContext(ctx).Save(variable).Error
}

func (r *WorkflowRepository) GetWorkflowVariable(ctx context.Context, workflowID, key string) (*workflow.WorkflowVariable, error) {
	var v workflow.WorkflowVariable
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND key = ?", workflowID, key).
		First(&v).Error
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func (r *WorkflowRepository) ListWorkflowVariables(ctx context.Context, workflowID string) ([]*workflow.WorkflowVariable, error) {
	var vars []*workflow.WorkflowVariable
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Find(&vars).Error
	if err != nil {
		return nil, err
	}

	return vars, nil
}

func (r *WorkflowRepository) DeleteWorkflowVariable(ctx context.Context, workflowID, key string) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("workflow_id = ? AND key = ?", workflowID, key).
		Delete(&workflow.WorkflowVariable{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// Environments

func (r *WorkflowRepository) CountEnvironments(ctx context.Context, workflowID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&workflow.Environment{}).
		Where("workflow_id = ?", workflowID).
		Count(&count).Error
	return count, err
}

func (r *WorkflowRepository) CreateEnvironment(ctx context.Context, env *workflow.Environment) error {
	return r.db.WithContext(ctx).Create(env).Error
}

func (r *WorkflowRepository) GetEnvironment(ctx context.Context, workflowID, envID string) (*workflow.Environment, error) {
	var env workflow.Environment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		First(&env).Error
	if err != nil {
		return nil, err
	}

	return &env, nil
}

func (r *WorkflowRepository) ListEnvironments(ctx context.Context, workflowID string) ([]*workflow.Environment, error) {
	var envs []*workflow.Environment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Find(&envs).Error
	if err != nil {
		return nil, err
	}

	return envs, nil
}

func (r *WorkflowRepository) UpdateEnvironment(ctx context.Context, workflowID, envID string, updates map[string]interface{}) (int64, error) {
	updates["updated_at"] = time.Now().Format(time.RFC3339)

	result := r.db.WithContext(ctx).
		Model(&workflow.Environment{}).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		Updates(updates)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func (r *WorkflowRepository) DeleteEnvironment(ctx context.Context, env *workflow.Environment) error {
	return r.db.WithContext(ctx).Delete(env).Error
}

func (r *WorkflowRepository) SetDefaultEnvironment(ctx context.Context, workflowID, envID string) (int64, error) {
	var updated int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&workflow.Environment{}).
			Where("workflow_id = ?", workflowID).
			Update("is_default", false).Error; err != nil {
			return err
		}

		result := tx.Model(&workflow.Environment{}).
			Where("workflow_id = ? AND id = ?", workflowID, envID).
			Update("is_default", true)
		if result.Error != nil {
			return result.Error
		}

		updated = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}

	return updated, nil
}
