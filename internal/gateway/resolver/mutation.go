package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Login authenticates a user
func (r *mutationResolver) Login(ctx context.Context, input LoginInput) (*AuthPayload, error) {
	url := fmt.Sprintf("%s/api/v1/auth/login", r.baseURLs["auth"])

	body, _ := json.Marshal(input)
	resp, err := r.clients.AuthClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid credentials")
	}

	var payload AuthPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &payload, nil
}

// Register creates a new user
func (r *mutationResolver) Register(ctx context.Context, input RegisterInput) (*AuthPayload, error) {
	url := fmt.Sprintf("%s/api/v1/auth/register", r.baseURLs["auth"])

	body, _ := json.Marshal(input)
	resp, err := r.clients.AuthClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed")
	}

	var payload AuthPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &payload, nil
}

// CreateWorkflow creates a new workflow
func (r *mutationResolver) CreateWorkflow(ctx context.Context, input CreateWorkflowInput) (*Workflow, error) {
	url := fmt.Sprintf("%s/api/v1/workflows", r.baseURLs["workflow"])

	body, _ := json.Marshal(input)
	resp, err := r.clients.WorkflowClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create workflow")
	}

	var workflow Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflow); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &workflow, nil
}

// UpdateWorkflow updates an existing workflow
func (r *mutationResolver) UpdateWorkflow(ctx context.Context, id string, input UpdateWorkflowInput) (*Workflow, error) {
	url := fmt.Sprintf("%s/api/v1/workflows/%s", r.baseURLs["workflow"], id)

	body, _ := json.Marshal(input)
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.clients.WorkflowClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update workflow")
	}

	var workflow Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflow); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &workflow, nil
}

// DeleteWorkflow deletes a workflow
func (r *mutationResolver) DeleteWorkflow(ctx context.Context, id string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/workflows/%s", r.baseURLs["workflow"], id)

	req, _ := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	resp, err := r.clients.WorkflowClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to delete workflow: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK, nil
}

// ExecuteWorkflow executes a workflow
func (r *mutationResolver) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*Execution, error) {
	url := fmt.Sprintf("%s/api/v1/workflows/%s/execute", r.baseURLs["execution"], workflowID)

	body, _ := json.Marshal(map[string]interface{}{"input": input})
	resp, err := r.clients.ExecutionClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to execute workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to execute workflow")
	}

	var execution Execution
	if err := json.NewDecoder(resp.Body).Decode(&execution); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &execution, nil
}

// CancelExecution cancels an execution
func (r *mutationResolver) CancelExecution(ctx context.Context, id string) (*Execution, error) {
	url := fmt.Sprintf("%s/api/v1/executions/%s/cancel", r.baseURLs["execution"], id)

	resp, err := r.clients.ExecutionClient.Post(url, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel execution: %w", err)
	}
	defer resp.Body.Close()

	var execution Execution
	if err := json.NewDecoder(resp.Body).Decode(&execution); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &execution, nil
}

// CreateCredential creates a new credential
func (r *mutationResolver) CreateCredential(ctx context.Context, input CreateCredentialInput) (*Credential, error) {
	url := fmt.Sprintf("%s/api/v1/credentials", r.baseURLs["credential"])

	body, _ := json.Marshal(input)
	resp, err := r.clients.CredentialClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create credential")
	}

	var credential Credential
	if err := json.NewDecoder(resp.Body).Decode(&credential); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &credential, nil
}

// DeleteCredential deletes a credential
func (r *mutationResolver) DeleteCredential(ctx context.Context, id string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/credentials/%s", r.baseURLs["credential"], id)

	req, _ := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	resp, err := r.clients.CredentialClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to delete credential: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK, nil
}

// CreateSchedule creates a new schedule
func (r *mutationResolver) CreateSchedule(ctx context.Context, input CreateScheduleInput) (*Schedule, error) {
	url := fmt.Sprintf("%s/api/v1/schedules", r.baseURLs["schedule"])

	body, _ := json.Marshal(input)
	resp, err := r.clients.ScheduleClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create schedule")
	}

	var schedule Schedule
	if err := json.NewDecoder(resp.Body).Decode(&schedule); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &schedule, nil
}

// DeleteSchedule deletes a schedule
func (r *mutationResolver) DeleteSchedule(ctx context.Context, id string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/schedules/%s", r.baseURLs["schedule"], id)

	req, _ := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	resp, err := r.clients.ScheduleClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to delete schedule: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK, nil
}

// SetVariable sets a variable
func (r *mutationResolver) SetVariable(ctx context.Context, key string, value string, varType *VariableType) (*Variable, error) {
	url := fmt.Sprintf("%s/api/v1/variables", r.baseURLs["variable"])

	input := map[string]interface{}{
		"key":   key,
		"value": value,
	}
	if varType != nil {
		input["type"] = string(*varType)
	}

	body, _ := json.Marshal(input)
	resp, err := r.clients.VariableClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to set variable: %w", err)
	}
	defer resp.Body.Close()

	var variable Variable
	if err := json.NewDecoder(resp.Body).Decode(&variable); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &variable, nil
}

// DeleteVariable deletes a variable
func (r *mutationResolver) DeleteVariable(ctx context.Context, key string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/variables/%s", r.baseURLs["variable"], key)

	req, _ := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	resp, err := r.clients.VariableClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to delete variable: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK, nil
}
