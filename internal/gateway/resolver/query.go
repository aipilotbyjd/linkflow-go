package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Me returns the current user
func (r *queryResolver) Me(ctx context.Context) (*User, error) {
	// Get user from context (set by auth middleware)
	userID := ctx.Value("userID")
	if userID == nil {
		return nil, fmt.Errorf("unauthorized")
	}

	return r.User(ctx, userID.(string))
}

// User returns a user by ID
func (r *queryResolver) User(ctx context.Context, id string) (*User, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", r.baseURLs["auth"], id)

	resp, err := r.clients.AuthClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user not found")
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// Workflow returns a workflow by ID
func (r *queryResolver) Workflow(ctx context.Context, id string) (*Workflow, error) {
	url := fmt.Sprintf("%s/api/v1/workflows/%s", r.baseURLs["workflow"], id)

	resp, err := r.clients.WorkflowClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("workflow not found")
	}

	var workflow Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflow); err != nil {
		return nil, fmt.Errorf("failed to decode workflow: %w", err)
	}

	return &workflow, nil
}

// Workflows returns a list of workflows
func (r *queryResolver) Workflows(ctx context.Context, filter *WorkflowFilter, pagination *PaginationInput) (*WorkflowConnection, error) {
	url := fmt.Sprintf("%s/api/v1/workflows", r.baseURLs["workflow"])

	resp, err := r.clients.WorkflowClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflows: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data  []Workflow `json:"data"`
		Total int        `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	edges := make([]*WorkflowEdge, len(result.Data))
	for i := range result.Data {
		edges[i] = &WorkflowEdge{
			Node:   &result.Data[i],
			Cursor: result.Data[i].ID,
		}
	}

	return &WorkflowConnection{
		Edges:      edges,
		TotalCount: result.Total,
		PageInfo: &PageInfo{
			HasNextPage:     false,
			HasPreviousPage: false,
		},
	}, nil
}

// Execution returns an execution by ID
func (r *queryResolver) Execution(ctx context.Context, id string) (*Execution, error) {
	url := fmt.Sprintf("%s/api/v1/executions/%s", r.baseURLs["execution"], id)

	resp, err := r.clients.ExecutionClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch execution: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("execution not found")
	}

	var execution Execution
	if err := json.NewDecoder(resp.Body).Decode(&execution); err != nil {
		return nil, fmt.Errorf("failed to decode execution: %w", err)
	}

	return &execution, nil
}

// Executions returns a list of executions
func (r *queryResolver) Executions(ctx context.Context, filter *ExecutionFilter, pagination *PaginationInput) (*ExecutionConnection, error) {
	url := fmt.Sprintf("%s/api/v1/executions", r.baseURLs["execution"])

	resp, err := r.clients.ExecutionClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch executions: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data  []Execution `json:"data"`
		Total int         `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode executions: %w", err)
	}

	edges := make([]*ExecutionEdge, len(result.Data))
	for i := range result.Data {
		edges[i] = &ExecutionEdge{
			Node:   &result.Data[i],
			Cursor: result.Data[i].ID,
		}
	}

	return &ExecutionConnection{
		Edges:      edges,
		TotalCount: result.Total,
		PageInfo: &PageInfo{
			HasNextPage:     false,
			HasPreviousPage: false,
		},
	}, nil
}

// NodeTypes returns all available node types
func (r *queryResolver) NodeTypes(ctx context.Context) ([]*NodeType, error) {
	// Return built-in node types
	return []*NodeType{
		{Type: "http", Name: "HTTP Request", Category: "Core", Version: "1.0"},
		{Type: "database", Name: "Database", Category: "Core", Version: "1.0"},
		{Type: "transform", Name: "Transform", Category: "Core", Version: "1.0"},
		{Type: "if", Name: "IF", Category: "Flow", Version: "1.0"},
		{Type: "switch", Name: "Switch", Category: "Flow", Version: "1.0"},
		{Type: "loop", Name: "Loop", Category: "Flow", Version: "1.0"},
		{Type: "forEach", Name: "For Each", Category: "Flow", Version: "1.0"},
		{Type: "set", Name: "Set", Category: "Utility", Version: "1.0"},
		{Type: "function", Name: "Function", Category: "Utility", Version: "1.0"},
		{Type: "wait", Name: "Wait", Category: "Utility", Version: "1.0"},
		{Type: "dateTime", Name: "Date & Time", Category: "Utility", Version: "1.0"},
		{Type: "crypto", Name: "Crypto", Category: "Utility", Version: "1.0"},
		{Type: "json", Name: "JSON", Category: "Utility", Version: "1.0"},
		{Type: "math", Name: "Math", Category: "Utility", Version: "1.0"},
		{Type: "text", Name: "Text", Category: "Utility", Version: "1.0"},
		{Type: "email", Name: "Email", Category: "Integration", Version: "1.0"},
		{Type: "slack", Name: "Slack", Category: "Integration", Version: "1.0"},
		{Type: "discord", Name: "Discord", Category: "Integration", Version: "1.0"},
		{Type: "telegram", Name: "Telegram", Category: "Integration", Version: "1.0"},
		{Type: "webhookTrigger", Name: "Webhook Trigger", Category: "Trigger", Version: "1.0"},
		{Type: "scheduleTrigger", Name: "Schedule Trigger", Category: "Trigger", Version: "1.0"},
		{Type: "manualTrigger", Name: "Manual Trigger", Category: "Trigger", Version: "1.0"},
	}, nil
}

// Credentials returns all credentials
func (r *queryResolver) Credentials(ctx context.Context) ([]*Credential, error) {
	url := fmt.Sprintf("%s/api/v1/credentials", r.baseURLs["credential"])

	resp, err := r.clients.CredentialClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch credentials: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []Credential `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode credentials: %w", err)
	}

	credentials := make([]*Credential, len(result.Data))
	for i := range result.Data {
		credentials[i] = &result.Data[i]
	}

	return credentials, nil
}

// Schedules returns schedules
func (r *queryResolver) Schedules(ctx context.Context, workflowID *string) ([]*Schedule, error) {
	url := fmt.Sprintf("%s/api/v1/schedules", r.baseURLs["schedule"])
	if workflowID != nil {
		url = fmt.Sprintf("%s?workflowId=%s", url, *workflowID)
	}

	resp, err := r.clients.ScheduleClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schedules: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []Schedule `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode schedules: %w", err)
	}

	schedules := make([]*Schedule, len(result.Data))
	for i := range result.Data {
		schedules[i] = &result.Data[i]
	}

	return schedules, nil
}

// Webhooks returns webhooks
func (r *queryResolver) Webhooks(ctx context.Context, workflowID *string) ([]*Webhook, error) {
	url := fmt.Sprintf("%s/api/v1/webhooks", r.baseURLs["webhook"])
	if workflowID != nil {
		url = fmt.Sprintf("%s?workflowId=%s", url, *workflowID)
	}

	resp, err := r.clients.WebhookClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch webhooks: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []Webhook `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode webhooks: %w", err)
	}

	webhooks := make([]*Webhook, len(result.Data))
	for i := range result.Data {
		webhooks[i] = &result.Data[i]
	}

	return webhooks, nil
}

// Variables returns all variables
func (r *queryResolver) Variables(ctx context.Context) ([]*Variable, error) {
	url := fmt.Sprintf("%s/api/v1/variables", r.baseURLs["variable"])

	resp, err := r.clients.VariableClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch variables: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []Variable `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode variables: %w", err)
	}

	variables := make([]*Variable, len(result.Data))
	for i := range result.Data {
		variables[i] = &result.Data[i]
	}

	return variables, nil
}

// Dashboard returns analytics dashboard
func (r *queryResolver) Dashboard(ctx context.Context) (*Dashboard, error) {
	url := fmt.Sprintf("%s/api/v1/dashboard", r.baseURLs["analytics"])

	resp, err := r.clients.AnalyticsClient.Get(url)
	if err != nil {
		// Return default dashboard if analytics service is unavailable
		return &Dashboard{
			TotalWorkflows:   0,
			ActiveWorkflows:  0,
			TotalExecutions:  0,
			SuccessRate:      0,
			AvgExecutionTime: 0,
			ExecutionsByDay:  []*DailyCount{},
			TopWorkflows:     []*WorkflowSummary{},
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var dashboard Dashboard
	if err := json.Unmarshal(body, &dashboard); err != nil {
		return nil, fmt.Errorf("failed to decode dashboard: %w", err)
	}

	return &dashboard, nil
}
