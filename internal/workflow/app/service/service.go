package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/workflow/adapters/templates"
	"github.com/linkflow-go/internal/workflow/ports"
	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrInvalidWorkflow  = errors.New("invalid workflow")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrWorkflowInactive = errors.New("workflow is inactive")
	ErrTemplateNotFound = errors.New("template not found")
)

type WorkflowService struct {
	repo              ports.WorkflowRepository
	eventBus          events.EventBus
	redis             *redis.Client
	logger            logger.Logger
	validationService *ValidationService
	triggerManager    ports.TriggerManager
	templateManager   ports.TemplateManager
	variableManager   *workflow.VariableManager
}

func NewWorkflowService(
	repo ports.WorkflowRepository,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
	triggerManager ports.TriggerManager,
	templateManager ports.TemplateManager,
) *WorkflowService {
	return &WorkflowService{
		repo:              repo,
		eventBus:          eventBus,
		redis:             redis,
		logger:            logger,
		validationService: NewValidationService(redis, logger),
		triggerManager:    triggerManager,
		templateManager:   templateManager,
		variableManager:   workflow.NewVariableManager(),
	}
}

func (s *WorkflowService) CheckReady() error {
	// Check database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repo.Ping(ctx); err != nil {
		return err
	}

	// Check Redis connection
	if err := s.redis.Ping(ctx).Err(); err != nil {
		return err
	}

	return nil
}

func (s *WorkflowService) ListWorkflows(ctx context.Context, userID string, page, limit int, status string) ([]*workflow.Workflow, int64, error) {
	opts := ports.ListWorkflowsOptions{
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
	previousVersion := wf.Version

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
			"previous_version": previousVersion,
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
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	versions, err := s.repo.ListVersions(ctx, workflowID)
	if err != nil {
		s.logger.Error("Failed to list workflow versions", "workflow_id", workflowID, "error", err)
		return nil, err
	}

	// Convert to interface slice
	result := make([]interface{}, len(versions))
	for i, v := range versions {
		result[i] = v
	}

	return result, nil
}

func (s *WorkflowService) GetWorkflowVersion(ctx context.Context, workflowID string, version int, userID string) (*workflow.Workflow, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	// Get specific version
	wv, err := s.repo.GetVersion(ctx, workflowID, version)
	if err != nil {
		s.logger.Error("Failed to get workflow version", "workflow_id", workflowID, "version", version, "error", err)
		return nil, err
	}

	// Parse workflow from version data
	var wf workflow.Workflow
	if err := json.Unmarshal([]byte(wv.Data), &wf); err != nil {
		s.logger.Error("Failed to parse workflow version data", "error", err)
		return nil, err
	}

	return &wf, nil
}

func (s *WorkflowService) CreateWorkflowVersion(ctx context.Context, workflowID, userID string, req *workflow.CreateVersionRequest) (int, error) {
	// Get current workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return 0, ErrWorkflowNotFound
	}

	// Create new version using repository
	changeNote := req.Message
	if changeNote == "" {
		changeNote = "Manual version created"
	}

	if err := s.repo.UpdateWithVersion(ctx, wf, changeNote); err != nil {
		s.logger.Error("Failed to create workflow version", "error", err)
		return 0, err
	}

	s.logger.Info("Workflow version created", "workflow_id", workflowID, "version", wf.Version)
	return wf.Version, nil
}

func (s *WorkflowService) RollbackWorkflowVersion(ctx context.Context, workflowID string, version int, userID string) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	// Restore to specific version
	if err := s.repo.RestoreVersion(ctx, workflowID, version, userID); err != nil {
		s.logger.Error("Failed to rollback workflow version", "workflow_id", workflowID, "version", version, "error", err)
		return err
	}

	// Publish event
	event := events.Event{
		Type: "workflow.version.rollback",
		Payload: map[string]interface{}{
			"workflow_id": workflowID,
			"version":     version,
			"user_id":     userID,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish rollback event", "error", err)
	}

	s.logger.Info("Workflow rolled back", "workflow_id", workflowID, "version", version)
	return nil
}

func (s *WorkflowService) ActivateWorkflow(ctx context.Context, workflowID, userID string) error {
	// Get workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return ErrWorkflowNotFound
	}

	// Validate workflow before activation
	if len(wf.Nodes) > 0 {
		if err := wf.Validate(); err != nil {
			s.logger.Error("Workflow validation failed during activation", "error", err)
			return ErrInvalidWorkflow
		}
	}

	// Activate workflow
	if err := wf.Activate(); err != nil {
		return err
	}

	// Update in database
	if err := s.repo.UpdateWorkflow(ctx, wf); err != nil {
		s.logger.Error("Failed to activate workflow", "error", err)
		return err
	}

	// Activate associated triggers
	triggers, _ := s.triggerManager.ListTriggers(ctx, workflowID)
	for _, trigger := range triggers {
		if trigger.Status == workflow.TriggerStatusInactive {
			if err := s.triggerManager.ActivateTrigger(ctx, trigger.ID); err != nil {
				s.logger.Warn("Failed to activate trigger", "trigger_id", trigger.ID, "error", err)
			}
		}
	}

	// Publish event
	event := events.Event{
		Type: "workflow.activated",
		Payload: map[string]interface{}{
			"workflow_id": workflowID,
			"user_id":     userID,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish activation event", "error", err)
	}

	s.logger.Info("Workflow activated", "workflow_id", workflowID)
	return nil
}

func (s *WorkflowService) DeactivateWorkflow(ctx context.Context, workflowID, userID string) error {
	// Get workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return ErrWorkflowNotFound
	}

	// Deactivate workflow
	wf.Deactivate()

	// Update in database
	if err := s.repo.UpdateWorkflow(ctx, wf); err != nil {
		s.logger.Error("Failed to deactivate workflow", "error", err)
		return err
	}

	// Deactivate associated triggers
	triggers, _ := s.triggerManager.ListTriggers(ctx, workflowID)
	for _, trigger := range triggers {
		if trigger.Status == workflow.TriggerStatusActive {
			if err := s.triggerManager.DeactivateTrigger(ctx, trigger.ID); err != nil {
				s.logger.Warn("Failed to deactivate trigger", "trigger_id", trigger.ID, "error", err)
			}
		}
	}

	// Publish event
	event := events.Event{
		Type: "workflow.deactivated",
		Payload: map[string]interface{}{
			"workflow_id": workflowID,
			"user_id":     userID,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish deactivation event", "error", err)
	}

	s.logger.Info("Workflow deactivated", "workflow_id", workflowID)
	return nil
}

func (s *WorkflowService) DuplicateWorkflow(ctx context.Context, workflowID, userID, name string) (*workflow.Workflow, error) {
	// Get original workflow
	original, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}

	// Clone workflow
	clone := original.Clone(name)
	clone.UserID = userID

	// Save clone
	if err := s.repo.CreateWorkflow(ctx, clone); err != nil {
		s.logger.Error("Failed to duplicate workflow", "error", err)
		return nil, err
	}

	// Publish event
	event := events.Event{
		Type: "workflow.duplicated",
		Payload: map[string]interface{}{
			"original_id": workflowID,
			"clone_id":    clone.ID,
			"user_id":     userID,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish duplication event", "error", err)
	}

	s.logger.Info("Workflow duplicated", "original_id", workflowID, "clone_id", clone.ID)
	return clone, nil
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
	// Get workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return "", ErrWorkflowNotFound
	}

	// Check if workflow is active
	if !wf.IsActive {
		return "", ErrWorkflowInactive
	}

	// Generate execution ID
	executionID := uuid.New().String()

	// Publish execution request event
	event := events.Event{
		Type:        "execution.requested",
		AggregateID: executionID,
		Payload: map[string]interface{}{
			"execution_id": executionID,
			"workflow_id":  workflowID,
			"user_id":      userID,
			"input_data":   data,
			"version":      wf.Version,
		},
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish execution request", "error", err)
		return "", err
	}

	s.logger.Info("Workflow execution requested", "execution_id", executionID, "workflow_id", workflowID)
	return executionID, nil
}

func (s *WorkflowService) TestWorkflow(ctx context.Context, workflowID, userID string, data map[string]interface{}) (interface{}, error) {
	// Get workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}

	// Validate workflow
	errors, warnings, validationErr := s.validationService.ValidateWorkflow(ctx, wf)

	// Build test result
	result := map[string]interface{}{
		"workflow_id": workflowID,
		"valid":       validationErr == nil,
		"errors":      errors,
		"warnings":    warnings,
		"node_count":  len(wf.Nodes),
		"input_data":  data,
		"test_mode":   true,
	}

	// If valid, simulate execution order
	if validationErr == nil {
		order, _ := s.validationService.GetExecutionOrder(ctx, wf)
		result["execution_order"] = order
		result["complexity"] = s.validationService.AnalyzeComplexity(ctx, wf)
	}

	return result, nil
}

func (s *WorkflowService) GetWorkflowPermissions(ctx context.Context, workflowID, userID string) ([]interface{}, error) {
	// Verify workflow exists
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	permissions, err := s.repo.ListWorkflowPermissions(ctx, workflowID)
	if err != nil {
		s.logger.Error("Failed to get workflow permissions", "error", err)
		return nil, err
	}

	result := make([]interface{}, len(permissions))
	for i, p := range permissions {
		result[i] = p
	}

	return result, nil
}

func (s *WorkflowService) ShareWorkflow(ctx context.Context, workflowID, userID, targetUserID, permission string) error {
	// Verify workflow exists and user is owner
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return ErrWorkflowNotFound
	}

	if wf.UserID != userID {
		return ErrUnauthorized
	}

	// Create permission record
	perm := map[string]interface{}{
		"id":          uuid.New().String(),
		"workflow_id": workflowID,
		"user_id":     targetUserID,
		"permission":  permission,
		"granted_by":  userID,
		"created_at":  time.Now(),
	}

	if err := s.repo.CreateWorkflowPermission(ctx, perm); err != nil {
		s.logger.Error("Failed to share workflow", "error", err)
		return err
	}

	s.logger.Info("Workflow shared", "workflow_id", workflowID, "target_user", targetUserID, "permission", permission)
	return nil
}

func (s *WorkflowService) UnshareWorkflow(ctx context.Context, workflowID, userID, targetUserID string) error {
	// Verify workflow exists and user is owner
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return ErrWorkflowNotFound
	}

	if wf.UserID != userID {
		return ErrUnauthorized
	}

	// Delete permission record
	_, err = s.repo.DeleteWorkflowPermission(ctx, workflowID, targetUserID)
	if err != nil {
		s.logger.Error("Failed to unshare workflow", "error", err)
		return err
	}

	s.logger.Info("Workflow unshared", "workflow_id", workflowID, "target_user", targetUserID)
	return nil
}

func (s *WorkflowService) PublishWorkflow(ctx context.Context, workflowID, userID, description string, tags []string) error {
	// Get workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return ErrWorkflowNotFound
	}

	// Create template from workflow
	template := &templates.Template{
		Name:        wf.Name,
		Description: description,
		Category:    "custom",
		Tags:        tags,
		CreatorID:   userID,
		IsPublic:    true,
	}

	wfJSON, _ := wf.ToJSON()
	template.Workflow = []byte(wfJSON)

	if err := s.templateManager.CreateTemplate(ctx, template); err != nil {
		s.logger.Error("Failed to publish workflow", "error", err)
		return err
	}

	s.logger.Info("Workflow published", "workflow_id", workflowID, "template_id", template.ID)
	return nil
}

func (s *WorkflowService) ImportWorkflow(ctx context.Context, userID string, data interface{}, format string) (*workflow.Workflow, error) {
	var wf *workflow.Workflow

	switch format {
	case "json":
		// Parse JSON data
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		wf = &workflow.Workflow{}
		if err := json.Unmarshal(jsonData, wf); err != nil {
			return nil, err
		}
	case "n8n":
		// Convert n8n format to LinkFlow format
		wf = convertN8NWorkflow(data)
	default:
		return nil, errors.New("unsupported import format")
	}

	// Generate new ID and set user
	wf.ID = uuid.New().String()
	wf.UserID = userID
	wf.Status = workflow.StatusInactive
	wf.IsActive = false
	wf.Version = 1
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()

	// Save workflow
	if err := s.repo.CreateWorkflow(ctx, wf); err != nil {
		s.logger.Error("Failed to import workflow", "error", err)
		return nil, err
	}

	s.logger.Info("Workflow imported", "workflow_id", wf.ID, "format", format)
	return wf, nil
}

func (s *WorkflowService) ExportWorkflow(ctx context.Context, workflowID, userID, format string) (interface{}, error) {
	// Get workflow
	wf, err := s.repo.GetWorkflow(ctx, workflowID, userID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}

	switch format {
	case "json":
		return wf, nil
	case "n8n":
		return convertToN8NFormat(wf), nil
	default:
		return wf, nil
	}
}

func (s *WorkflowService) GetWorkflowStats(ctx context.Context, workflowID, userID string) (interface{}, error) {
	// Verify workflow exists
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}
	stats, err := s.repo.GetWorkflowStats(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *WorkflowService) GetWorkflowExecutions(ctx context.Context, workflowID, userID string, page, limit int) ([]interface{}, int64, error) {
	// Verify workflow exists
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, 0, ErrWorkflowNotFound
	}
	offset := (page - 1) * limit
	executions, total, err := s.repo.ListWorkflowExecutions(ctx, workflowID, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	result := make([]interface{}, len(executions))
	for i, e := range executions {
		result[i] = e
	}

	return result, total, nil
}

func (s *WorkflowService) GetLatestRun(ctx context.Context, workflowID, userID string) (interface{}, error) {
	// Verify workflow exists
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	exec, err := s.repo.GetLatestWorkflowExecution(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	return exec, nil
}

func (s *WorkflowService) ListCategories(ctx context.Context) ([]interface{}, error) {
	categories := s.templateManager.GetCategories()
	result := make([]interface{}, len(categories))
	for i, c := range categories {
		result[i] = c
	}
	return result, nil
}

func (s *WorkflowService) CreateCategory(ctx context.Context, name, description, icon string) (interface{}, error) {
	category := map[string]interface{}{
		"id":          uuid.New().String(),
		"name":        name,
		"description": description,
		"icon":        icon,
		"created_at":  time.Now(),
	}

	if err := s.repo.CreateCategory(ctx, category); err != nil {
		s.logger.Error("Failed to create category", "error", err)
		return nil, err
	}

	return category, nil
}

func (s *WorkflowService) SearchWorkflows(ctx context.Context, userID, query, category string, tags []string, page, limit int) ([]*workflow.Workflow, int64, error) {
	opts := ports.ListWorkflowsOptions{
		UserID: userID,
		Search: query,
		Tags:   tags,
		Page:   page,
		Limit:  limit,
	}

	return s.repo.ListWorkflows(ctx, opts)
}

func (s *WorkflowService) GetPopularTags(ctx context.Context, limit int) ([]string, error) {
	tags, err := s.repo.GetPopularTags(ctx, limit)
	if err != nil {
		return []string{}, nil
	}

	return tags, nil
}

// Helper functions for import/export
func convertN8NWorkflow(data interface{}) *workflow.Workflow {
	// Convert n8n workflow format to LinkFlow format
	wf := workflow.NewWorkflow("Imported Workflow", "Imported from n8n", "")

	if n8nData, ok := data.(map[string]interface{}); ok {
		if name, ok := n8nData["name"].(string); ok {
			wf.Name = name
		}
		// Add more conversion logic as needed
	}

	return wf
}

func convertToN8NFormat(wf *workflow.Workflow) map[string]interface{} {
	// Convert LinkFlow workflow to n8n format
	return map[string]interface{}{
		"name":        wf.Name,
		"nodes":       wf.Nodes,
		"connections": wf.Connections,
		"settings":    wf.Settings,
	}
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
	if err := s.repo.SaveWorkflowVariable(ctx, variable); err != nil {
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

	variable, err := s.repo.GetWorkflowVariable(ctx, workflowID, key)
	if err != nil {
		return nil, workflow.ErrVariableNotFound
	}

	return variable, nil
}

// ListWorkflowVariables lists all variables for a workflow
func (s *WorkflowService) ListWorkflowVariables(ctx context.Context, workflowID, userID string) ([]*workflow.WorkflowVariable, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	return s.repo.ListWorkflowVariables(ctx, workflowID)
}

// DeleteWorkflowVariable deletes a workflow variable
func (s *WorkflowService) DeleteWorkflowVariable(ctx context.Context, workflowID, userID, key string) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	rows, err := s.repo.DeleteWorkflowVariable(ctx, workflowID, key)
	if err != nil {
		return err
	}

	if rows == 0 {
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
	count, err := s.repo.CountEnvironments(ctx, workflowID)
	if err != nil {
		return err
	}
	if count == 0 {
		env.IsDefault = true
	}

	// Save to database
	if err := s.repo.CreateEnvironment(ctx, env); err != nil {
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

	env, err := s.repo.GetEnvironment(ctx, workflowID, envID)
	if err != nil {
		return nil, err
	}

	return env, nil
}

// ListEnvironments lists all environments for a workflow
func (s *WorkflowService) ListEnvironments(ctx context.Context, workflowID, userID string) ([]*workflow.Environment, error) {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return nil, ErrWorkflowNotFound
	}

	return s.repo.ListEnvironments(ctx, workflowID)
}

// UpdateEnvironment updates an environment
func (s *WorkflowService) UpdateEnvironment(ctx context.Context, workflowID, userID, envID string, updates map[string]interface{}) error {
	// Verify workflow exists and user has permission
	if _, err := s.repo.GetWorkflow(ctx, workflowID, userID); err != nil {
		return ErrWorkflowNotFound
	}

	rows, err := s.repo.UpdateEnvironment(ctx, workflowID, envID, updates)
	if err != nil {
		return err
	}

	if rows == 0 {
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
	env, err := s.repo.GetEnvironment(ctx, workflowID, envID)
	if err != nil {
		return err
	}

	if env.IsDefault {
		return errors.New("cannot delete default environment")
	}

	// Delete environment
	if err := s.repo.DeleteEnvironment(ctx, env); err != nil {
		return err
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

	rows, err := s.repo.SetDefaultEnvironment(ctx, workflowID, envID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("environment not found")
	}

	s.logger.Info("Default environment set", "id", envID, "workflow_id", workflowID)
	return nil
}
