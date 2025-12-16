package resolver

import (
	"time"

	credentialDomain "github.com/linkflow-go/pkg/contracts/credential"
	executionDomain "github.com/linkflow-go/pkg/contracts/execution"
	notificationDomain "github.com/linkflow-go/pkg/contracts/notification"
	scheduleDomain "github.com/linkflow-go/pkg/contracts/schedule"
	userDomain "github.com/linkflow-go/pkg/contracts/user"
	webhookDomain "github.com/linkflow-go/pkg/contracts/webhook"
	workflowDomain "github.com/linkflow-go/pkg/contracts/workflow"
)

// GraphQL DTO types - these are separate from domain models to allow
// GraphQL-specific formatting, nullable fields, and API versioning.
// Use the conversion functions below to convert between domain and DTO types.

// User represents a user in GraphQL responses
type User struct {
	ID               string    `json:"id"`
	Email            string    `json:"email"`
	Username         string    `json:"username"`
	FirstName        *string   `json:"firstName"`
	LastName         *string   `json:"lastName"`
	Avatar           *string   `json:"avatar"`
	EmailVerified    bool      `json:"emailVerified"`
	TwoFactorEnabled bool      `json:"twoFactorEnabled"`
	Roles            []string  `json:"roles"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// Workflow represents a workflow
type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Nodes       []*Node           `json:"nodes"`
	Connections []*Connection     `json:"connections"`
	Settings    *WorkflowSettings `json:"settings"`
	Status      WorkflowStatus    `json:"status"`
	IsActive    bool              `json:"isActive"`
	Version     int               `json:"version"`
	Tags        []string          `json:"tags"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// Node represents a workflow node
type Node struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Position   *Position              `json:"position"`
	Parameters map[string]interface{} `json:"parameters"`
	Disabled   bool                   `json:"disabled"`
}

// Connection represents a connection between nodes
type Connection struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	SourcePort *string `json:"sourcePort"`
	TargetPort *string `json:"targetPort"`
}

// Position represents node position
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// WorkflowSettings represents workflow settings
type WorkflowSettings struct {
	Timeout        int    `json:"timeout"`
	RetryOnFailure bool   `json:"retryOnFailure"`
	MaxRetries     int    `json:"maxRetries"`
	Timezone       string `json:"timezone"`
}

// Execution represents a workflow execution
type Execution struct {
	ID             string                 `json:"id"`
	WorkflowID     string                 `json:"workflowId"`
	Version        int                    `json:"version"`
	Status         ExecutionStatus        `json:"status"`
	StartedAt      *time.Time             `json:"startedAt"`
	FinishedAt     *time.Time             `json:"finishedAt"`
	ExecutionTime  *int                   `json:"executionTime"`
	Data           map[string]interface{} `json:"data"`
	Error          *string                `json:"error"`
	NodeExecutions []*NodeExecution       `json:"nodeExecutions"`
	CreatedAt      time.Time              `json:"createdAt"`
}

// NodeExecution represents a node execution
type NodeExecution struct {
	ID         string                 `json:"id"`
	NodeID     string                 `json:"nodeId"`
	NodeType   string                 `json:"nodeType"`
	Status     ExecutionStatus        `json:"status"`
	StartedAt  *time.Time             `json:"startedAt"`
	FinishedAt *time.Time             `json:"finishedAt"`
	InputData  map[string]interface{} `json:"inputData"`
	OutputData map[string]interface{} `json:"outputData"`
	Error      *string                `json:"error"`
	RetryCount int                    `json:"retryCount"`
}

// NodeType represents a node type definition
type NodeType struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description *string     `json:"description"`
	Category    string      `json:"category"`
	Icon        *string     `json:"icon"`
	Version     string      `json:"version"`
	Schema      *NodeSchema `json:"schema"`
}

// NodeSchema represents node input/output schema
type NodeSchema struct {
	Inputs  []*SchemaField `json:"inputs"`
	Outputs []*SchemaField `json:"outputs"`
}

// SchemaField represents a schema field
type SchemaField struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Label    string      `json:"label"`
	Required bool        `json:"required"`
	Default  interface{} `json:"default"`
}

// Credential represents a credential
type Credential struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description *string    `json:"description"`
	IsShared    bool       `json:"isShared"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Schedule represents a schedule
type Schedule struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	WorkflowID     string     `json:"workflowId"`
	CronExpression string     `json:"cronExpression"`
	Timezone       string     `json:"timezone"`
	IsActive       bool       `json:"isActive"`
	LastRunAt      *time.Time `json:"lastRunAt"`
	NextRunAt      *time.Time `json:"nextRunAt"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// Webhook represents a webhook
type Webhook struct {
	ID           string     `json:"id"`
	WorkflowID   string     `json:"workflowId"`
	Path         string     `json:"path"`
	Method       string     `json:"method"`
	URL          string     `json:"url"`
	IsActive     bool       `json:"isActive"`
	LastCalledAt *time.Time `json:"lastCalledAt"`
	CallCount    int        `json:"callCount"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// Variable represents a variable
type Variable struct {
	Key         string       `json:"key"`
	Value       string       `json:"value"`
	Type        VariableType `json:"type"`
	Description *string      `json:"description"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// Dashboard represents analytics dashboard
type Dashboard struct {
	TotalWorkflows   int                `json:"totalWorkflows"`
	ActiveWorkflows  int                `json:"activeWorkflows"`
	TotalExecutions  int                `json:"totalExecutions"`
	SuccessRate      float64            `json:"successRate"`
	AvgExecutionTime float64            `json:"avgExecutionTime"`
	ExecutionsByDay  []*DailyCount      `json:"executionsByDay"`
	TopWorkflows     []*WorkflowSummary `json:"topWorkflows"`
}

// DailyCount represents daily execution count
type DailyCount struct {
	Date    string `json:"date"`
	Count   int    `json:"count"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// WorkflowSummary represents workflow summary
type WorkflowSummary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ExecutionCount int     `json:"executionCount"`
	SuccessRate    float64 `json:"successRate"`
}

// AuthPayload represents authentication response
type AuthPayload struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
	User         *User  `json:"user"`
}

// ExecutionUpdate represents execution update event
type ExecutionUpdate struct {
	ExecutionID string                 `json:"executionId"`
	Status      ExecutionStatus        `json:"status"`
	NodeID      *string                `json:"nodeId"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Notification represents a notification
type Notification struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"createdAt"`
}

// Connections for pagination
type WorkflowConnection struct {
	Edges      []*WorkflowEdge `json:"edges"`
	PageInfo   *PageInfo       `json:"pageInfo"`
	TotalCount int             `json:"totalCount"`
}

type WorkflowEdge struct {
	Node   *Workflow `json:"node"`
	Cursor string    `json:"cursor"`
}

type ExecutionConnection struct {
	Edges      []*ExecutionEdge `json:"edges"`
	PageInfo   *PageInfo        `json:"pageInfo"`
	TotalCount int              `json:"totalCount"`
}

type ExecutionEdge struct {
	Node   *Execution `json:"node"`
	Cursor string     `json:"cursor"`
}

type PageInfo struct {
	HasNextPage     bool    `json:"hasNextPage"`
	HasPreviousPage bool    `json:"hasPreviousPage"`
	StartCursor     *string `json:"startCursor"`
	EndCursor       *string `json:"endCursor"`
}

// Enums
type WorkflowStatus string

const (
	WorkflowStatusActive   WorkflowStatus = "ACTIVE"
	WorkflowStatusInactive WorkflowStatus = "INACTIVE"
	WorkflowStatusError    WorkflowStatus = "ERROR"
)

type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "PENDING"
	ExecutionStatusQueued    ExecutionStatus = "QUEUED"
	ExecutionStatusRunning   ExecutionStatus = "RUNNING"
	ExecutionStatusPaused    ExecutionStatus = "PAUSED"
	ExecutionStatusCompleted ExecutionStatus = "COMPLETED"
	ExecutionStatusFailed    ExecutionStatus = "FAILED"
	ExecutionStatusCancelled ExecutionStatus = "CANCELLED"
	ExecutionStatusTimeout   ExecutionStatus = "TIMEOUT"
)

type VariableType string

const (
	VariableTypeString  VariableType = "STRING"
	VariableTypeNumber  VariableType = "NUMBER"
	VariableTypeBoolean VariableType = "BOOLEAN"
	VariableTypeSecret  VariableType = "SECRET"
)

// Input types
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterInput struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type CreateWorkflowInput struct {
	Name        string             `json:"name"`
	Description *string            `json:"description"`
	Nodes       []*NodeInput       `json:"nodes"`
	Connections []*ConnectionInput `json:"connections"`
	Tags        []string           `json:"tags"`
}

type UpdateWorkflowInput struct {
	Name        *string            `json:"name"`
	Description *string            `json:"description"`
	Nodes       []*NodeInput       `json:"nodes"`
	Connections []*ConnectionInput `json:"connections"`
	Tags        []string           `json:"tags"`
}

type NodeInput struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Position   *PositionInput         `json:"position"`
	Parameters map[string]interface{} `json:"parameters"`
	Disabled   *bool                  `json:"disabled"`
}

type ConnectionInput struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	SourcePort *string `json:"sourcePort"`
	TargetPort *string `json:"targetPort"`
}

type PositionInput struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type CreateCredentialInput struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Data        map[string]interface{} `json:"data"`
	Description *string                `json:"description"`
}

type CreateScheduleInput struct {
	Name           string                 `json:"name"`
	WorkflowID     string                 `json:"workflowId"`
	CronExpression string                 `json:"cronExpression"`
	Timezone       *string                `json:"timezone"`
	Data           map[string]interface{} `json:"data"`
}

type PaginationInput struct {
	First  *int    `json:"first"`
	After  *string `json:"after"`
	Last   *int    `json:"last"`
	Before *string `json:"before"`
}

type WorkflowFilter struct {
	Search   *string         `json:"search"`
	Status   *WorkflowStatus `json:"status"`
	IsActive *bool           `json:"isActive"`
	Tags     []string        `json:"tags"`
}

type ExecutionFilter struct {
	WorkflowID *string          `json:"workflowId"`
	Status     *ExecutionStatus `json:"status"`
	DateFrom   *time.Time       `json:"dateFrom"`
	DateTo     *time.Time       `json:"dateTo"`
}

// Conversion functions from domain models to GraphQL DTOs

// UserFromDomain converts a domain user to GraphQL DTO
func UserFromDomain(u *userDomain.User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:               u.ID,
		Email:            u.Email,
		Username:         u.Username,
		FirstName:        strPtr(u.FirstName),
		LastName:         strPtr(u.LastName),
		Avatar:           strPtr(u.Avatar),
		EmailVerified:    u.EmailVerified,
		TwoFactorEnabled: u.TwoFactorEnabled,
		Roles:            u.GetRoleNames(),
		CreatedAt:        u.CreatedAt,
		UpdatedAt:        u.UpdatedAt,
	}
}

// WorkflowFromDomain converts a domain workflow to GraphQL DTO
func WorkflowFromDomain(w *workflowDomain.Workflow) *Workflow {
	if w == nil {
		return nil
	}

	nodes := make([]*Node, len(w.Nodes))
	for i, n := range w.Nodes {
		nodes[i] = &Node{
			ID:         n.ID,
			Name:       n.Name,
			Type:       n.Type,
			Position:   &Position{X: n.Position.X, Y: n.Position.Y},
			Parameters: n.Parameters,
			Disabled:   n.Disabled,
		}
	}

	connections := make([]*Connection, len(w.Connections))
	for i, c := range w.Connections {
		connections[i] = &Connection{
			ID:         c.ID,
			Source:     c.Source,
			Target:     c.Target,
			SourcePort: strPtr(c.SourcePort),
			TargetPort: strPtr(c.TargetPort),
		}
	}

	return &Workflow{
		ID:          w.ID,
		Name:        w.Name,
		Description: strPtr(w.Description),
		Nodes:       nodes,
		Connections: connections,
		Settings: &WorkflowSettings{
			Timeout:        w.Settings.Timeout,
			RetryOnFailure: w.Settings.RetryOnFailure,
			MaxRetries:     w.Settings.MaxRetries,
			Timezone:       w.Settings.Timezone,
		},
		Status:    WorkflowStatus(w.Status),
		IsActive:  w.IsActive,
		Version:   w.Version,
		Tags:      w.Tags,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
}

// ExecutionFromDomain converts a domain execution to GraphQL DTO
func ExecutionFromDomain(e *executionDomain.Execution) *Execution {
	if e == nil {
		return nil
	}
	return &Execution{
		ID:            e.ID,
		WorkflowID:    e.WorkflowID,
		Version:       e.Version,
		Status:        ExecutionStatus(e.Status),
		StartedAt:     e.StartedAt,
		FinishedAt:    e.FinishedAt,
		ExecutionTime: toIntPtr(int(e.ExecutionTime)),
		Data:          e.OutputData,
		Error:         strPtr(e.Error),
		CreatedAt:     e.CreatedAt,
	}
}

// CredentialFromDomain converts a domain credential to GraphQL DTO
func CredentialFromDomain(c *credentialDomain.Credential) *Credential {
	if c == nil {
		return nil
	}
	return &Credential{
		ID:          c.ID,
		Name:        c.Name,
		Type:        c.Type,
		Description: strPtr(c.Description),
		IsShared:    c.IsShared,
		LastUsedAt:  c.LastUsedAt,
		CreatedAt:   c.CreatedAt,
	}
}

// ScheduleFromDomain converts a domain schedule to GraphQL DTO
func ScheduleFromDomain(s *scheduleDomain.Schedule) *Schedule {
	if s == nil {
		return nil
	}
	return &Schedule{
		ID:             s.ID,
		Name:           s.Name,
		WorkflowID:     s.WorkflowID,
		CronExpression: s.CronExpression,
		Timezone:       s.Timezone,
		IsActive:       s.IsActive,
		LastRunAt:      s.LastRunAt,
		NextRunAt:      s.NextRunAt,
		CreatedAt:      s.CreatedAt,
	}
}

// WebhookFromDomain converts a domain webhook to GraphQL DTO
func WebhookFromDomain(w *webhookDomain.Webhook) *Webhook {
	if w == nil {
		return nil
	}
	return &Webhook{
		ID:           w.ID,
		WorkflowID:   w.WorkflowID,
		Path:         w.Path,
		Method:       w.Method,
		URL:          w.GetURL(""),
		IsActive:     w.IsActive,
		LastCalledAt: w.LastCalledAt,
		CallCount:    int(w.CallCount),
		CreatedAt:    w.CreatedAt,
	}
}

// NotificationFromDomain converts a domain notification to GraphQL DTO
func NotificationFromDomain(n *notificationDomain.Notification) *Notification {
	if n == nil {
		return nil
	}
	return &Notification{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Subject,
		Message:   n.Body,
		Data:      n.Data,
		CreatedAt: n.CreatedAt,
	}
}

// Helper functions
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toIntPtr(i int) *int {
	return &i
}
