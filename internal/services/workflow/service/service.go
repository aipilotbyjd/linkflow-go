package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/internal/services/workflow/repository"
	"github.com/linkflow-go/internal/services/workflow/templates"
	"github.com/linkflow-go/internal/services/workflow/triggers"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrInvalidWorkflow  = errors.New("invalid workflow")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrWorkflowInactive = errors.New("workflow is inactive")
	ErrTemplateNotFound = errors.New("template not found")
)

type WorkflowService struct {
	repo              *repository.WorkflowRepository
	eventBus          events.EventBus
	redis             *redis.Client
	logger            logger.Logger
	validationService *ValidationService
	triggerManager    *triggers.TriggerManager
	templateManager   *templates.TemplateManager
	variableManager   *workflow.VariableManager
	db                *database.DB
}

func NewWorkflowService(
	repo *repository.WorkflowRepository,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
	db *database.DB,
) *WorkflowService {
	return &WorkflowService{
		repo:              repo,
		eventBus:          eventBus,
		redis:             redis,
		logger:            logger,
		validationService: NewValidationService(redis, logger),
		triggerManager:    triggers.NewTriggerManager(db, redis, eventBus, logger),
		templateManager:   templates.NewTemplateManager(db, logger),
		variableManager:   workflow.NewVariableManager(),
		db:                db,
	}
}

func (s *WorkflowService) CheckReady() error {
	return nil
}

func (s *WorkflowService) ListWorkflows(ctx context.Context, userID string, page, limit int, status string) ([]*workflow.Workflow, int64, error) {
	opts := repository.ListWorkflowsOptions{
		UserID: userID,
		Page:   page,
		Limit:  limit,
		Status: status,
	}
	return s.repo.ListWorkflows(ctx, opts)
}

func (s *WorkflowService) GetWorkflow(ctx context.Context, workflowID, userID string) (*workflow.Workflow, error) {
	return s.repo.GetWorkflow(ctx, workflowID, userID)
}

func (s *WorkflowService) CreateWorkflow(ctx context.Context, req *workflow.CreateWorkflowRequest) (*workflow.Workflow, error) {
	// Validate workflow structure
	if req.Name == "" {
		return nil, ErrInvalidWorkflow
	}

	// Create new workflow
	wf := workflow.NewWorkflow(req.Name, req.Description, req.UserID)

	// Set nodes and connections if provided
	if req.Nodes != nil {
		wf.Nodes = req.Nodes
	}
	if req.Connections != nil {
		wf.Connections = req.Connections
	}
	if req.Tags != nil {
		wf.Tags = req.Tags
	}

	// Validate workflow structure (DAG validation)
	if len(wf.Nodes) > 0 {
		if err := wf.Validate(); err != nil {
			s.logger.Error("Workflow validation failed", "error", err)
			return nil, ErrInvalidWorkflow
		}
	}

	// Store in database
	if err := s.repo.CreateWorkflow(ctx, wf); err != nil {
		s.logger.Error("Failed to create workflow", "error", err)
		return nil, err
	}

	// Publish WorkflowCreated event
	event := events.Event{
		Type: "workflow.created",
		Payload: map[string]interface{}{
			"workflow_id": wf.ID,
			"user_id":     wf.UserID,
			"name":        wf.Name,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish workflow created event", "error", err)
	}

	s.logger.Info("Workflow created", "id", wf.ID, "user", wf.UserID)
	return wf, nil
}

func (s *WorkflowService) UpdateWorkflow(ctx context.Context, req *workflow.UpdateWorkflowRequest) (*workflow.Workflow, error) {
	// Get existing workflow
	wf, err := s.repo.GetWorkflow(ctx, req.WorkflowID, req.UserID)
	if err != nil {
		s.logger.Error("Workflow not found", "id", req.WorkflowID, "error", err)
		return nil, ErrWorkflowNotFound
	}

	// Check version for optimistic locking
	if req.Version > 0 && wf.Version != req.Version {
		s.logger.Warn("Version mismatch", "expected", req.Version, "actual", wf.Version)
		return nil, errors.New("version mismatch - workflow was modified by another user")
	}

	// Store previous version for history
	previousData, _ := wf.ToJSON()
	version := &workflow.WorkflowVersion{
		ID:         wf.ID + "_v" + strconv.Itoa(wf.Version),
		WorkflowID: wf.ID,
		Version:    wf.Version,
		Data:       previousData,
		ChangedBy:  req.UserID,
		ChangeNote: "Updated workflow",
		CreatedAt:  time.Now(),
	}
	// In a real implementation, we'd save this version to a versions table

	// Update workflow fields
	if req.Name != "" {
		wf.Name = req.Name
	}
	if req.Description != "" {
		wf.Description = req.Description
	}
	if req.Nodes != nil {
		wf.Nodes = req.Nodes
	}
	if req.Connections != nil {
		wf.Connections = req.Connections
	}
	if req.Tags != nil {
		wf.Tags = req.Tags
	}

	// Increment version
	wf.Version++
	wf.UpdatedAt = time.Now()

	// Validate updated workflow
	if len(wf.Nodes) > 0 {
		if err := wf.Validate(); err != nil {
			s.logger.Error("Workflow validation failed", "error", err)
			return nil, ErrInvalidWorkflow
		}
	}

	// Save to database
	if err := s.repo.UpdateWorkflow(ctx, wf); err != nil {
		s.logger.Error("Failed to update workflow", "error", err)
		return nil, err
	}

	// Publish WorkflowUpdated event
	event := events.Event{
		Type: "workflow.updated",
		Payload: map[string]interface{}{
			"workflow_id":      wf.ID,
			"user_id":          wf.UserID,
			"version":          wf.Version,
			"previous_version": version.Version,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish workflow updated event", "error", err)
	}

	s.logger.Info("Workflow updated", "id", wf.ID, "version", wf.Version)
	return wf, nil
}

func (s *WorkflowService) DeleteWorkflow(ctx context.Context, workflowID, userID string) error {
	// Check if workflow exists before deletion
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		s.logger.Error("Workflow not found for deletion", "id", workflowID, "error", err)
		return ErrWorkflowNotFound
	}

	// Perform soft delete in database
	if err := s.repo.DeleteWorkflow(ctx, workflowID, userID); err != nil {
		s.logger.Error("Failed to delete workflow", "error", err)
		return err
	}

	// Publish WorkflowDeleted event
	event := events.Event{
		Type: "workflow.deleted",
		Payload: map[string]interface{}{
			"workflow_id": workflowID,
			"user_id":     userID,
			"name":        wf.Name,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish workflow deleted event", "error", err)
	}

	s.logger.Info("Workflow deleted", "id", workflowID, "user", userID)
	return nil
}

func (s *WorkflowService) GetWorkflowVersions(ctx context.Context, workflowID, userID string) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (s *WorkflowService) GetWorkflowVersion(ctx context.Context, workflowID string, version int, userID string) (*workflow.Workflow, error) {
	return &workflow.Workflow{}, nil
}

func (s *WorkflowService) CreateWorkflowVersion(ctx context.Context, workflowID, userID string, req *workflow.CreateVersionRequest) (int, error) {
	return 1, nil
}

func (s *WorkflowService) RollbackWorkflowVersion(ctx context.Context, workflowID string, version int, userID string) error {
	return nil
}

func (s *WorkflowService) ActivateWorkflow(ctx context.Context, workflowID, userID string) error {
	return nil
}

func (s *WorkflowService) DeactivateWorkflow(ctx context.Context, workflowID, userID string) error {
	return nil
}

func (s *WorkflowService) DuplicateWorkflow(ctx context.Context, workflowID, userID, name string) (*workflow.Workflow, error) {
	return &workflow.Workflow{}, nil
}

func (s *WorkflowService) ValidateWorkflow(ctx context.Context, workflowID, userID string) ([]string, []string, error) {
	// Get the workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		s.logger.Error("Failed to get workflow for validation", "id", workflowID, "error", err)
		return nil, nil, ErrWorkflowNotFound
	}

	// Perform comprehensive validation
	errors, warnings, err := s.validationService.ValidateWorkflow(ctx, wf)

	// Also validate DAG structure
	if err == nil {
		if dagErr := s.validationService.ValidateDAG(ctx, wf); dagErr != nil {
			errors = append(errors, dagErr.Error())
			err = dagErr
		}
	}

	// Publish validation event
	event := events.Event{
		Type: "workflow.validated",
		Payload: map[string]interface{}{
			"workflow_id": workflowID,
			"valid":       err == nil,
			"errors":      len(errors),
			"warnings":    len(warnings),
		},
	}
	if pubErr := s.eventBus.Publish(ctx, event); pubErr != nil {
		s.logger.Warn("Failed to publish validation event", "error", pubErr)
	}

	return errors, warnings, err
}

func (s *WorkflowService) ExecuteWorkflow(ctx context.Context, workflowID, userID string, data map[string]interface{}) (string, error) {
	return "exec_id", nil
}

func (s *WorkflowService) TestWorkflow(ctx context.Context, workflowID, userID string, data map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (s *WorkflowService) GetWorkflowPermissions(ctx context.Context, workflowID, userID string) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (s *WorkflowService) ShareWorkflow(ctx context.Context, workflowID, userID, targetUserID, permission string) error {
	return nil
}

func (s *WorkflowService) UnshareWorkflow(ctx context.Context, workflowID, userID, targetUserID string) error {
	return nil
}

func (s *WorkflowService) PublishWorkflow(ctx context.Context, workflowID, userID, description string, tags []string) error {
	return nil
}

func (s *WorkflowService) ImportWorkflow(ctx context.Context, userID string, data interface{}, format string) (*workflow.Workflow, error) {
	return &workflow.Workflow{}, nil
}

func (s *WorkflowService) ExportWorkflow(ctx context.Context, workflowID, userID, format string) (interface{}, error) {
	return nil, nil
}

func (s *WorkflowService) GetWorkflowStats(ctx context.Context, workflowID, userID string) (interface{}, error) {
	return nil, nil
}

func (s *WorkflowService) GetWorkflowExecutions(ctx context.Context, workflowID, userID string, page, limit int) ([]interface{}, int64, error) {
	return []interface{}{}, 0, nil
}

func (s *WorkflowService) GetLatestRun(ctx context.Context, workflowID, userID string) (interface{}, error) {
	return nil, nil
}

func (s *WorkflowService) ListCategories(ctx context.Context) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (s *WorkflowService) CreateCategory(ctx context.Context, name, description, icon string) (interface{}, error) {
	return nil, nil
}

func (s *WorkflowService) SearchWorkflows(ctx context.Context, userID, query, category string, tags []string, page, limit int) ([]*workflow.Workflow, int64, error) {
	return []*workflow.Workflow{}, 0, nil
}

func (s *WorkflowService) GetPopularTags(ctx context.Context, limit int) ([]string, error) {
	return []string{}, nil
}

func (s *WorkflowService) HandleExecutionCompleted(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling execution completed for workflow stats")
	return nil
}

func (s *WorkflowService) HandleExecutionFailed(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling execution failed for workflow stats")
	return nil
}

func (s *WorkflowService) HandleNodeUpdated(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling node updated for workflow validation")
	return nil
}

// Trigger management methods

// CreateTrigger creates a new trigger for a workflow
func (s *WorkflowService) CreateTrigger(ctx context.Context, workflowID, userID string, config map[string]interface{}) (*workflow.WorkflowTrigger, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	// Create trigger
	trigger, err := s.triggerManager.CreateTrigger(ctx, workflowID, config)
	if err != nil {
		s.logger.Error("Failed to create trigger", "workflow_id", workflowID, "error", err)
		return nil, err
	}

	s.logger.Info("Trigger created", "trigger_id", trigger.ID, "workflow_id", workflowID, "type", trigger.Type)
	return trigger, nil
}

// GetTrigger gets a trigger by ID
func (s *WorkflowService) GetTrigger(ctx context.Context, triggerID, userID string) (*workflow.WorkflowTrigger, error) {
	trigger, err := s.triggerManager.GetTrigger(ctx, triggerID)
	if err != nil {
		return nil, err
	}

	// Verify user has permission to view this trigger's workflow
	if _, err := s.repo.GetWorkflow(ctx, trigger.WorkflowID, userID); err != nil {
		return nil, ErrUnauthorized
	}

	return trigger, nil
}

// ListTriggers lists all triggers for a workflow
func (s *WorkflowService) ListTriggers(ctx context.Context, workflowID, userID string) ([]*workflow.WorkflowTrigger, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	return s.triggerManager.ListTriggers(ctx, workflowID)
}

// UpdateTrigger updates a trigger
func (s *WorkflowService) UpdateTrigger(ctx context.Context, triggerID, userID string, updates map[string]interface{}) (*workflow.WorkflowTrigger, error) {
	// Get trigger to check workflow
	trigger, err := s.triggerManager.GetTrigger(ctx, triggerID)
	if err != nil {
		return nil, err
	}

	// Verify user has permission
	if _, err := s.repo.GetWorkflow(ctx, trigger.WorkflowID, userID); err != nil {
		return nil, ErrUnauthorized
	}

	// Update trigger
	updatedTrigger, err := s.triggerManager.UpdateTrigger(ctx, triggerID, updates)
	if err != nil {
		s.logger.Error("Failed to update trigger", "trigger_id", triggerID, "error", err)
		return nil, err
	}

	s.logger.Info("Trigger updated", "trigger_id", triggerID)
	return updatedTrigger, nil
}

// DeleteTrigger deletes a trigger
func (s *WorkflowService) DeleteTrigger(ctx context.Context, triggerID, userID string) error {
	// Get trigger to check workflow
	trigger, err := s.triggerManager.GetTrigger(ctx, triggerID)
	if err != nil {
		return err
	}

	// Verify user has permission
	if _, err := s.repo.GetWorkflow(ctx, trigger.WorkflowID, userID); err != nil {
		return ErrUnauthorized
	}

	// Delete trigger
	if err := s.triggerManager.DeleteTrigger(ctx, triggerID); err != nil {
		s.logger.Error("Failed to delete trigger", "trigger_id", triggerID, "error", err)
		return err
	}

	s.logger.Info("Trigger deleted", "trigger_id", triggerID)
	return nil
}

// ActivateTrigger activates a trigger
func (s *WorkflowService) ActivateTrigger(ctx context.Context, triggerID, userID string) error {
	// Get trigger to check workflow
	trigger, err := s.triggerManager.GetTrigger(ctx, triggerID)
	if err != nil {
		return err
	}

	// Verify user has permission
	wf, err := s.repo.GetWorkflow(ctx, trigger.WorkflowID, userID)
	if err != nil {
		return ErrUnauthorized
	}

	// Check if workflow is active
	if !wf.IsActive {
		return ErrWorkflowInactive
	}

	// Activate trigger
	if err := s.triggerManager.ActivateTrigger(ctx, triggerID); err != nil {
		s.logger.Error("Failed to activate trigger", "trigger_id", triggerID, "error", err)
		return err
	}

	s.logger.Info("Trigger activated", "trigger_id", triggerID)
	return nil
}

// DeactivateTrigger deactivates a trigger
func (s *WorkflowService) DeactivateTrigger(ctx context.Context, triggerID, userID string) error {
	// Get trigger to check workflow
	trigger, err := s.triggerManager.GetTrigger(ctx, triggerID)
	if err != nil {
		return err
	}

	// Verify user has permission
	if _, err := s.repo.GetWorkflow(ctx, trigger.WorkflowID, userID); err != nil {
		return ErrUnauthorized
	}

	// Deactivate trigger
	if err := s.triggerManager.DeactivateTrigger(ctx, triggerID); err != nil {
		s.logger.Error("Failed to deactivate trigger", "trigger_id", triggerID, "error", err)
		return err
	}

	s.logger.Info("Trigger deactivated", "trigger_id", triggerID)
	return nil
}

// TestTrigger tests a trigger with sample data
func (s *WorkflowService) TestTrigger(ctx context.Context, triggerID, userID string, testData map[string]interface{}) (map[string]interface{}, error) {
	// Get trigger to check workflow
	trigger, err := s.triggerManager.GetTrigger(ctx, triggerID)
	if err != nil {
		return nil, err
	}

	// Verify user has permission
	if _, err := s.repo.GetWorkflow(ctx, trigger.WorkflowID, userID); err != nil {
		return nil, ErrUnauthorized
	}

	// Test trigger
	result, err := s.triggerManager.TestTrigger(ctx, triggerID, testData)
	if err != nil {
		s.logger.Error("Failed to test trigger", "trigger_id", triggerID, "error", err)
		return nil, err
	}

	s.logger.Info("Trigger tested", "trigger_id", triggerID, "would_fire", result["would_fire"])
	return result, nil
}

// StartTriggerManager starts the trigger manager service
func (s *WorkflowService) StartTriggerManager(ctx context.Context) error {
	return s.triggerManager.Start(ctx)
}

// StopTriggerManager stops the trigger manager service
func (s *WorkflowService) StopTriggerManager(ctx context.Context) error {
	return s.triggerManager.Stop(ctx)
}

// Template management methods

// CreateTemplate creates a new workflow template
func (s *WorkflowService) CreateTemplate(ctx context.Context, req *workflow.CreateTemplateRequest) (*templates.Template, error) {
	template := &templates.Template{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Icon:        req.Icon,
		Tags:        req.Tags,
		CreatorID:   req.CreatorID,
		IsPublic:    false,
	}

	// Convert workflow to JSON
	wfJSON, err := req.Workflow.ToJSON()
	if err != nil {
		return nil, err
	}
	template.Workflow = []byte(wfJSON)

	// Create template
	if err := s.templateManager.CreateTemplate(ctx, template); err != nil {
		s.logger.Error("Failed to create template", "error", err)
		return nil, err
	}

	s.logger.Info("Template created", "id", template.ID, "name", template.Name)
	return template, nil
}

// ListTemplates lists available templates
func (s *WorkflowService) ListTemplates(ctx context.Context, category string) ([]*templates.Template, error) {
	templates, err := s.templateManager.ListTemplates(ctx, category, nil)
	if err != nil {
		s.logger.Error("Failed to list templates", "error", err)
		return nil, err
	}
	return templates, nil
}

// GetTemplate gets a template by ID
func (s *WorkflowService) GetTemplate(ctx context.Context, templateID string) (*templates.Template, error) {
	template, err := s.templateManager.GetTemplate(ctx, templateID)
	if err != nil {
		if err == templates.ErrTemplateNotFound {
			return nil, ErrTemplateNotFound
		}
		s.logger.Error("Failed to get template", "id", templateID, "error", err)
		return nil, err
	}
	return template, nil
}

// CreateFromTemplate creates a workflow from a template
func (s *WorkflowService) CreateFromTemplate(ctx context.Context, templateID, userID, name string, variables map[string]interface{}) (*workflow.Workflow, error) {
	// Instantiate workflow from template
	wf, err := s.templateManager.InstantiateTemplate(ctx, templateID, userID, name, variables)
	if err != nil {
		s.logger.Error("Failed to instantiate template", "template_id", templateID, "error", err)
		return nil, err
	}

	// Save workflow to database
	if err := s.repo.CreateWorkflow(ctx, wf); err != nil {
		s.logger.Error("Failed to save workflow from template", "error", err)
		return nil, err
	}

	// Publish event
	event := events.Event{
		Type: "workflow.created_from_template",
		Payload: map[string]interface{}{
			"workflow_id": wf.ID,
			"template_id": templateID,
			"user_id":     userID,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish event", "error", err)
	}

	s.logger.Info("Workflow created from template", "workflow_id", wf.ID, "template_id", templateID)
	return wf, nil
}

// Variable and Environment management methods

// SetWorkflowVariable sets a workflow variable
func (s *WorkflowService) SetWorkflowVariable(ctx context.Context, workflowID, userID string, variable *workflow.WorkflowVariable) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	// Validate variable
	if err := workflow.ValidateVariableName(variable.Key); err != nil {
		return err
	}

	variable.WorkflowID = workflowID
	variable.CreatedAt = time.Now().Format(time.RFC3339)
	variable.UpdatedAt = time.Now().Format(time.RFC3339)

	// Save to database
	if err := s.db.WithContext(ctx).Save(variable).Error; err != nil {
		s.logger.Error("Failed to save workflow variable", "error", err)
		return err
	}

	// Update in-memory manager
	s.variableManager.SetVariable(workflowID, variable)

	s.logger.Info("Workflow variable set", "workflow_id", workflowID, "key", variable.Key)
	return nil
}

// GetWorkflowVariable gets a workflow variable
func (s *WorkflowService) GetWorkflowVariable(ctx context.Context, workflowID, userID, key string) (*workflow.WorkflowVariable, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	// Get from database
	var variable workflow.WorkflowVariable
	err := s.db.WithContext(ctx).
		Where("workflow_id = ? AND key = ?", workflowID, key).
		First(&variable).Error
	if err != nil {
		return nil, workflow.ErrVariableNotFound
	}

	return &variable, nil
}

// ListWorkflowVariables lists all variables for a workflow
func (s *WorkflowService) ListWorkflowVariables(ctx context.Context, workflowID, userID string) ([]*workflow.WorkflowVariable, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	// Get from database
	var variables []*workflow.WorkflowVariable
	err := s.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Find(&variables).Error
	if err != nil {
		return nil, err
	}

	return variables, nil
}

// DeleteWorkflowVariable deletes a workflow variable
func (s *WorkflowService) DeleteWorkflowVariable(ctx context.Context, workflowID, userID, key string) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	// Delete from database
	result := s.db.WithContext(ctx).
		Where("workflow_id = ? AND key = ?", workflowID, key).
		Delete(&workflow.WorkflowVariable{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return workflow.ErrVariableNotFound
	}

	// Remove from in-memory manager
	s.variableManager.DeleteVariable(workflowID, key)

	s.logger.Info("Workflow variable deleted", "workflow_id", workflowID, "key", key)
	return nil
}

// CreateEnvironment creates an environment for a workflow
func (s *WorkflowService) CreateEnvironment(ctx context.Context, workflowID, userID string, env *workflow.Environment) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	env.ID = uuid.New().String()
	env.WorkflowID = workflowID
	env.CreatedAt = time.Now().Format(time.RFC3339)
	env.UpdatedAt = time.Now().Format(time.RFC3339)

	// If this is the first environment, make it default
	var count int64
	s.db.Model(&workflow.Environment{}).Where("workflow_id = ?", workflowID).Count(&count)
	if count == 0 {
		env.IsDefault = true
	}

	// Save to database
	if err := s.db.WithContext(ctx).Create(env).Error; err != nil {
		s.logger.Error("Failed to create environment", "error", err)
		return err
	}

	// Update in-memory manager
	s.variableManager.SetEnvironment(workflowID, env)

	s.logger.Info("Environment created", "id", env.ID, "workflow_id", workflowID, "name", env.Name)
	return nil
}

// GetEnvironment gets an environment by ID
func (s *WorkflowService) GetEnvironment(ctx context.Context, workflowID, userID, envID string) (*workflow.Environment, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	var env workflow.Environment
	err := s.db.WithContext(ctx).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		First(&env).Error
	if err != nil {
		return nil, err
	}

	return &env, nil
}

// ListEnvironments lists all environments for a workflow
func (s *WorkflowService) ListEnvironments(ctx context.Context, workflowID, userID string) ([]*workflow.Environment, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	var environments []*workflow.Environment
	err := s.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Find(&environments).Error
	if err != nil {
		return nil, err
	}

	return environments, nil
}

// UpdateEnvironment updates an environment
func (s *WorkflowService) UpdateEnvironment(ctx context.Context, workflowID, userID, envID string, updates map[string]interface{}) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	updates["updated_at"] = time.Now().Format(time.RFC3339)

	result := s.db.WithContext(ctx).
		Model(&workflow.Environment{}).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("environment not found")
	}

	s.logger.Info("Environment updated", "id", envID, "workflow_id", workflowID)
	return nil
}

// DeleteEnvironment deletes an environment
func (s *WorkflowService) DeleteEnvironment(ctx context.Context, workflowID, userID, envID string) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	// Check if it's the default environment
	var env workflow.Environment
	err := s.db.WithContext(ctx).
		Where("workflow_id = ? AND id = ?", workflowID, envID).
		First(&env).Error
	if err != nil {
		return err
	}

	if env.IsDefault {
		return errors.New("cannot delete default environment")
	}

	// Delete environment
	result := s.db.WithContext(ctx).Delete(&env)
	if result.Error != nil {
		return result.Error
	}

	s.logger.Info("Environment deleted", "id", envID, "workflow_id", workflowID)
	return nil
}

// SetDefaultEnvironment sets the default environment for a workflow
func (s *WorkflowService) SetDefaultEnvironment(ctx context.Context, workflowID, userID, envID string) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	// Transaction to update default status
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove default from all environments
		if err := tx.Model(&workflow.Environment{}).
			Where("workflow_id = ?", workflowID).
			Update("is_default", false).Error; err != nil {
			return err
		}

		// Set new default
		result := tx.Model(&workflow.Environment{}).
			Where("workflow_id = ? AND id = ?", workflowID, envID).
			Update("is_default", true)

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return errors.New("environment not found")
		}

		return nil
	})

	if err != nil {
		return err
	}

	s.logger.Info("Default environment set", "id", envID, "workflow_id", workflowID)
	return nil
}
