package variables

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	repo     *Repository
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

func NewService(repo *Repository, eventBus events.EventBus, redis *redis.Client, logger logger.Logger) *Service {
	return &Service{
		repo:     repo,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

// Variable operations

func (s *Service) CreateVariable(ctx context.Context, req CreateVariableRequest) (*workflow.WorkflowVariable, error) {
	// Validate variable name
	if err := workflow.ValidateVariableName(req.Key); err != nil {
		return nil, err
	}

	// Check if variable already exists
	existing, _ := s.repo.GetVariable(ctx, req.WorkflowID, req.Key)
	if existing != nil {
		return nil, fmt.Errorf("variable '%s' already exists", req.Key)
	}

	// Determine type if not provided
	varType := req.Type
	if varType == "" {
		varType = workflow.ParseVariableType(req.Value)
	}

	variable := &workflow.WorkflowVariable{
		Key:         req.Key,
		WorkflowID:  req.WorkflowID,
		Name:        req.Name,
		Type:        varType,
		Value:       req.Value,
		Description: req.Description,
		Scope:       req.Scope,
		Environment: req.Environment,
		Encrypted:   req.Encrypted,
		ReadOnly:    req.ReadOnly,
		Required:    req.Required,
	}

	if variable.Scope == "" {
		variable.Scope = workflow.ScopeWorkflow
	}
	if variable.Name == "" {
		variable.Name = variable.Key
	}

	if err := s.repo.CreateVariable(ctx, variable); err != nil {
		return nil, fmt.Errorf("failed to create variable: %w", err)
	}

	// Invalidate cache
	s.invalidateCache(ctx, req.WorkflowID)

	// Publish event
	event := events.NewEventBuilder("variable.created").
		WithAggregateID(req.WorkflowID).
		WithPayload("key", req.Key).
		WithPayload("scope", variable.Scope).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Variable created", "workflowId", req.WorkflowID, "key", req.Key)
	return variable, nil
}

func (s *Service) GetVariable(ctx context.Context, workflowID, key string) (*workflow.WorkflowVariable, error) {
	return s.repo.GetVariable(ctx, workflowID, key)
}

func (s *Service) ListVariables(ctx context.Context, workflowID string) ([]*workflow.WorkflowVariable, error) {
	return s.repo.ListVariables(ctx, workflowID)
}

func (s *Service) ListVariablesByScope(ctx context.Context, workflowID, scope string) ([]*workflow.WorkflowVariable, error) {
	return s.repo.ListVariablesByScope(ctx, workflowID, scope)
}

func (s *Service) UpdateVariable(ctx context.Context, workflowID, key string, req UpdateVariableRequest) (*workflow.WorkflowVariable, error) {
	variable, err := s.repo.GetVariable(ctx, workflowID, key)
	if err != nil {
		return nil, err
	}

	if variable.ReadOnly {
		return nil, workflow.ErrVariableReadOnly
	}

	// Update fields
	if req.Name != "" {
		variable.Name = req.Name
	}
	if req.Value != nil {
		variable.Value = req.Value
		if req.Type == "" {
			variable.Type = workflow.ParseVariableType(req.Value)
		}
	}
	if req.Type != "" {
		variable.Type = req.Type
	}
	if req.Description != "" {
		variable.Description = req.Description
	}

	if err := s.repo.UpdateVariable(ctx, variable); err != nil {
		return nil, fmt.Errorf("failed to update variable: %w", err)
	}

	// Invalidate cache
	s.invalidateCache(ctx, workflowID)

	// Publish event
	event := events.NewEventBuilder("variable.updated").
		WithAggregateID(workflowID).
		WithPayload("key", key).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Variable updated", "workflowId", workflowID, "key", key)
	return variable, nil
}

func (s *Service) DeleteVariable(ctx context.Context, workflowID, key string) error {
	variable, err := s.repo.GetVariable(ctx, workflowID, key)
	if err != nil {
		return err
	}

	if variable.ReadOnly {
		return workflow.ErrVariableReadOnly
	}

	if err := s.repo.DeleteVariable(ctx, workflowID, key); err != nil {
		return fmt.Errorf("failed to delete variable: %w", err)
	}

	// Invalidate cache
	s.invalidateCache(ctx, workflowID)

	// Publish event
	event := events.NewEventBuilder("variable.deleted").
		WithAggregateID(workflowID).
		WithPayload("key", key).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Variable deleted", "workflowId", workflowID, "key", key)
	return nil
}

// Environment operations

func (s *Service) CreateEnvironment(ctx context.Context, req CreateEnvironmentRequest) (*workflow.Environment, error) {
	// Check if environment with same name exists
	existing, _ := s.repo.GetEnvironmentByName(ctx, req.WorkflowID, req.Name)
	if existing != nil {
		return nil, fmt.Errorf("environment '%s' already exists", req.Name)
	}

	env := &workflow.Environment{
		ID:          uuid.New().String(),
		WorkflowID:  req.WorkflowID,
		Name:        req.Name,
		Description: req.Description,
		Variables:   req.Variables,
		IsDefault:   req.IsDefault,
	}

	if env.Variables == nil {
		env.Variables = make(map[string]interface{})
	}

	// If this is the first environment or marked as default, set it as default
	envs, _ := s.repo.ListEnvironments(ctx, req.WorkflowID)
	if len(envs) == 0 || req.IsDefault {
		env.IsDefault = true
	}

	if err := s.repo.CreateEnvironment(ctx, env); err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	// If this is default, unset other defaults
	if env.IsDefault {
		s.repo.SetDefaultEnvironment(ctx, req.WorkflowID, env.ID)
	}

	s.logger.Info("Environment created", "workflowId", req.WorkflowID, "name", req.Name)
	return env, nil
}

func (s *Service) GetEnvironment(ctx context.Context, workflowID, envID string) (*workflow.Environment, error) {
	return s.repo.GetEnvironment(ctx, workflowID, envID)
}

func (s *Service) GetEnvironmentByName(ctx context.Context, workflowID, name string) (*workflow.Environment, error) {
	return s.repo.GetEnvironmentByName(ctx, workflowID, name)
}

func (s *Service) ListEnvironments(ctx context.Context, workflowID string) ([]*workflow.Environment, error) {
	return s.repo.ListEnvironments(ctx, workflowID)
}

func (s *Service) GetDefaultEnvironment(ctx context.Context, workflowID string) (*workflow.Environment, error) {
	return s.repo.GetDefaultEnvironment(ctx, workflowID)
}

func (s *Service) UpdateEnvironment(ctx context.Context, workflowID, envID string, req UpdateEnvironmentRequest) (*workflow.Environment, error) {
	env, err := s.repo.GetEnvironment(ctx, workflowID, envID)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		env.Name = req.Name
	}
	if req.Description != "" {
		env.Description = req.Description
	}
	if req.Variables != nil {
		env.Variables = req.Variables
	}

	if err := s.repo.UpdateEnvironment(ctx, env); err != nil {
		return nil, fmt.Errorf("failed to update environment: %w", err)
	}

	s.logger.Info("Environment updated", "workflowId", workflowID, "envId", envID)
	return env, nil
}

func (s *Service) SetDefaultEnvironment(ctx context.Context, workflowID, envID string) error {
	if err := s.repo.SetDefaultEnvironment(ctx, workflowID, envID); err != nil {
		return fmt.Errorf("failed to set default environment: %w", err)
	}

	s.logger.Info("Default environment set", "workflowId", workflowID, "envId", envID)
	return nil
}

func (s *Service) DeleteEnvironment(ctx context.Context, workflowID, envID string) error {
	env, err := s.repo.GetEnvironment(ctx, workflowID, envID)
	if err != nil {
		return err
	}

	if env.IsDefault {
		return fmt.Errorf("cannot delete default environment")
	}

	if err := s.repo.DeleteEnvironment(ctx, workflowID, envID); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	s.logger.Info("Environment deleted", "workflowId", workflowID, "envId", envID)
	return nil
}

// SetEnvironmentVariable sets a variable in an environment
func (s *Service) SetEnvironmentVariable(ctx context.Context, workflowID, envID, key string, value interface{}) error {
	env, err := s.repo.GetEnvironment(ctx, workflowID, envID)
	if err != nil {
		return err
	}

	if env.Variables == nil {
		env.Variables = make(map[string]interface{})
	}
	env.Variables[key] = value

	return s.repo.UpdateEnvironment(ctx, env)
}

// DeleteEnvironmentVariable removes a variable from an environment
func (s *Service) DeleteEnvironmentVariable(ctx context.Context, workflowID, envID, key string) error {
	env, err := s.repo.GetEnvironment(ctx, workflowID, envID)
	if err != nil {
		return err
	}

	delete(env.Variables, key)
	return s.repo.UpdateEnvironment(ctx, env)
}

// BuildVariableContext builds a variable context for execution
func (s *Service) BuildVariableContext(ctx context.Context, workflowID, environmentName string) (*workflow.VariableContext, error) {
	varCtx := workflow.NewVariableContext()

	// Load global variables
	globalVars, _ := s.repo.ListGlobalVariables(ctx)
	for _, v := range globalVars {
		varCtx.SetGlobalVariable(v.Key, v.Value)
		if v.ReadOnly {
			varCtx.MarkReadOnly(v.Key)
		}
		if v.Encrypted {
			varCtx.MarkEncrypted(v.Key)
		}
	}

	// Load workflow variables
	workflowVars, _ := s.repo.ListVariables(ctx, workflowID)
	for _, v := range workflowVars {
		varCtx.SetWorkflowVariable(v.Key, v.Value)
		if v.ReadOnly {
			varCtx.MarkReadOnly(v.Key)
		}
		if v.Encrypted {
			varCtx.MarkEncrypted(v.Key)
		}
	}

	// Load environment
	var env *workflow.Environment
	var err error
	if environmentName != "" {
		env, err = s.repo.GetEnvironmentByName(ctx, workflowID, environmentName)
	} else {
		env, err = s.repo.GetDefaultEnvironment(ctx, workflowID)
	}
	if err == nil && env != nil {
		varCtx.SetEnvironment(env)
	}

	return varCtx, nil
}

// Cache management
func (s *Service) invalidateCache(ctx context.Context, workflowID string) {
	cacheKey := fmt.Sprintf("variables:%s", workflowID)
	s.redis.Del(ctx, cacheKey)
}

// Request types

type CreateVariableRequest struct {
	WorkflowID  string      `json:"-"`
	Key         string      `json:"key" binding:"required"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Value       interface{} `json:"value" binding:"required"`
	Description string      `json:"description"`
	Scope       string      `json:"scope"`
	Environment string      `json:"environment"`
	Encrypted   bool        `json:"encrypted"`
	ReadOnly    bool        `json:"readOnly"`
	Required    bool        `json:"required"`
}

type UpdateVariableRequest struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
}

type CreateEnvironmentRequest struct {
	WorkflowID  string                 `json:"-"`
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Variables   map[string]interface{} `json:"variables"`
	IsDefault   bool                   `json:"isDefault"`
}

type UpdateEnvironmentRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Variables   map[string]interface{} `json:"variables"`
}

// CreateDefaultEnvironments creates default development, staging, and production environments
func (s *Service) CreateDefaultEnvironments(ctx context.Context, workflowID string) error {
	environments := []CreateEnvironmentRequest{
		{
			WorkflowID:  workflowID,
			Name:        "development",
			Description: "Development environment",
			IsDefault:   true,
			Variables:   map[string]interface{}{"ENV": "development", "DEBUG": true},
		},
		{
			WorkflowID:  workflowID,
			Name:        "staging",
			Description: "Staging environment",
			Variables:   map[string]interface{}{"ENV": "staging", "DEBUG": false},
		},
		{
			WorkflowID:  workflowID,
			Name:        "production",
			Description: "Production environment",
			Variables:   map[string]interface{}{"ENV": "production", "DEBUG": false},
		},
	}

	for _, req := range environments {
		if _, err := s.CreateEnvironment(ctx, req); err != nil {
			s.logger.Warn("Failed to create default environment", "name", req.Name, "error", err)
		}
	}

	return nil
}
