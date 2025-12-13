package variables

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// Variable operations

func (r *Repository) CreateVariable(ctx context.Context, variable *workflow.WorkflowVariable) error {
	variable.CreatedAt = time.Now().Format(time.RFC3339)
	variable.UpdatedAt = variable.CreatedAt
	return r.db.WithContext(ctx).Create(variable).Error
}

func (r *Repository) GetVariable(ctx context.Context, workflowID, key string) (*workflow.WorkflowVariable, error) {
	var variable workflow.WorkflowVariable
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND key = ?", workflowID, key).
		First(&variable).Error
	if err != nil {
		return nil, fmt.Errorf("variable not found: %w", err)
	}
	return &variable, nil
}

func (r *Repository) ListVariables(ctx context.Context, workflowID string) ([]*workflow.WorkflowVariable, error) {
	var variables []*workflow.WorkflowVariable
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Find(&variables).Error
	return variables, err
}

func (r *Repository) ListVariablesByScope(ctx context.Context, workflowID, scope string) ([]*workflow.WorkflowVariable, error) {
	var variables []*workflow.WorkflowVariable
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND scope = ?", workflowID, scope).
		Find(&variables).Error
	return variables, err
}

func (r *Repository) UpdateVariable(ctx context.Context, variable *workflow.WorkflowVariable) error {
	variable.UpdatedAt = time.Now().Format(time.RFC3339)
	return r.db.WithContext(ctx).Save(variable).Error
}

func (r *Repository) DeleteVariable(ctx context.Context, workflowID, key string) error {
	return r.db.WithContext(ctx).
		Where("workflow_id = ? AND key = ?", workflowID, key).
		Delete(&workflow.WorkflowVariable{}).Error
}

func (r *Repository) DeleteAllVariables(ctx context.Context, workflowID string) error {
	return r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Delete(&workflow.WorkflowVariable{}).Error
}

// Environment operations

func (r *Repository) CreateEnvironment(ctx context.Context, env *workflow.Environment) error {
	if env.ID == "" {
		env.ID = uuid.New().String()
	}
	env.CreatedAt = time.Now().Format(time.RFC3339)
	env.UpdatedAt = env.CreatedAt
	return r.db.WithContext(ctx).Create(env).Error
}

func (r *Repository) GetEnvironment(ctx context.Context, workflowID, envID string) (*workflow.Environment, error) {
	var env workflow.Environment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		First(&env).Error
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}
	return &env, nil
}

func (r *Repository) GetEnvironmentByName(ctx context.Context, workflowID, name string) (*workflow.Environment, error) {
	var env workflow.Environment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND name = ?", workflowID, name).
		First(&env).Error
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}
	return &env, nil
}

func (r *Repository) ListEnvironments(ctx context.Context, workflowID string) ([]*workflow.Environment, error) {
	var environments []*workflow.Environment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Find(&environments).Error
	return environments, err
}

func (r *Repository) GetDefaultEnvironment(ctx context.Context, workflowID string) (*workflow.Environment, error) {
	var env workflow.Environment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND is_default = ?", workflowID, true).
		First(&env).Error
	if err != nil {
		return nil, fmt.Errorf("default environment not found: %w", err)
	}
	return &env, nil
}

func (r *Repository) UpdateEnvironment(ctx context.Context, env *workflow.Environment) error {
	env.UpdatedAt = time.Now().Format(time.RFC3339)
	return r.db.WithContext(ctx).Save(env).Error
}

func (r *Repository) SetDefaultEnvironment(ctx context.Context, workflowID, envID string) error {
	// First, unset all defaults
	if err := r.db.WithContext(ctx).
		Model(&workflow.Environment{}).
		Where("workflow_id = ?", workflowID).
		Update("is_default", false).Error; err != nil {
		return err
	}

	// Set the new default
	return r.db.WithContext(ctx).
		Model(&workflow.Environment{}).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		Update("is_default", true).Error
}

func (r *Repository) DeleteEnvironment(ctx context.Context, workflowID, envID string) error {
	return r.db.WithContext(ctx).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		Delete(&workflow.Environment{}).Error
}

// Global variables (workflowID = "global")

func (r *Repository) ListGlobalVariables(ctx context.Context) ([]*workflow.WorkflowVariable, error) {
	var variables []*workflow.WorkflowVariable
	err := r.db.WithContext(ctx).
		Where("scope = ?", workflow.ScopeGlobal).
		Find(&variables).Error
	return variables, err
}
