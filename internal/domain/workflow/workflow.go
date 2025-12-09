package workflow

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Workflow struct {
	ID          string       `json:"id" gorm:"primaryKey"`
	Name        string       `json:"name" gorm:"not null"`
	Description string       `json:"description"`
	UserID      string       `json:"userId" gorm:"not null;index"`
	TeamID      string       `json:"teamId" gorm:"index"`
	Nodes       []Node       `json:"nodes" gorm:"serializer:json"`
	Connections []Connection `json:"connections" gorm:"serializer:json"`
	Settings    Settings     `json:"settings" gorm:"serializer:json"`
	Status      string       `json:"status" gorm:"default:'inactive'"`
	IsActive    bool         `json:"isActive" gorm:"default:false"`
	Version     int          `json:"version" gorm:"default:1"`
	Tags        []string     `json:"tags" gorm:"serializer:json"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type Node struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Position   Position               `json:"position"`
	Parameters map[string]interface{} `json:"parameters"`
	Disabled   bool                   `json:"disabled"`
	RetryCount int                    `json:"retryCount"`
	Timeout    int                    `json:"timeout"`
}

type Connection struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	SourcePort string `json:"sourcePort"`
	TargetPort string `json:"targetPort"`
	Data       map[string]interface{} `json:"data"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Settings struct {
	ErrorHandling   ErrorHandling   `json:"errorHandling"`
	Timeout         int             `json:"timeout"`
	RetryOnFailure  bool            `json:"retryOnFailure"`
	MaxRetries      int             `json:"maxRetries"`
	SaveDataOnError bool            `json:"saveDataOnError"`
	Timezone        string          `json:"timezone"`
}

type ErrorHandling struct {
	ContinueOnFail bool   `json:"continueOnFail"`
	RetryInterval  int    `json:"retryInterval"`
	MaxRetries     int    `json:"maxRetries"`
	ErrorWorkflow  string `json:"errorWorkflow"`
}

type WorkflowVersion struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	WorkflowID string    `json:"workflowId" gorm:"not null;index"`
	Version    int       `json:"version" gorm:"not null"`
	Data       string    `json:"data" gorm:"type:jsonb"`
	ChangedBy  string    `json:"changedBy"`
	ChangeNote string    `json:"changeNote"`
	CreatedAt  time.Time `json:"createdAt"`
}

type WorkflowExecution struct {
	ID             string                 `json:"id" gorm:"primaryKey"`
	WorkflowID     string                 `json:"workflowId" gorm:"not null;index"`
	Version        int                    `json:"version"`
	Status         string                 `json:"status" gorm:"default:'pending'"`
	StartedAt      time.Time              `json:"startedAt"`
	FinishedAt     *time.Time             `json:"finishedAt"`
	ExecutionTime  int64                  `json:"executionTime"`
	Data           map[string]interface{} `json:"data" gorm:"serializer:json"`
	Error          string                 `json:"error"`
	NodeExecutions []NodeExecution        `json:"nodeExecutions" gorm:"foreignKey:ExecutionID"`
	CreatedBy      string                 `json:"createdBy"`
	CreatedAt      time.Time              `json:"createdAt"`
}

type NodeExecution struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	ExecutionID  string                 `json:"executionId" gorm:"not null;index"`
	NodeID       string                 `json:"nodeId" gorm:"not null"`
	Status       string                 `json:"status"`
	StartedAt    time.Time              `json:"startedAt"`
	FinishedAt   *time.Time             `json:"finishedAt"`
	InputData    map[string]interface{} `json:"inputData" gorm:"serializer:json"`
	OutputData   map[string]interface{} `json:"outputData" gorm:"serializer:json"`
	Error        string                 `json:"error"`
	RetryCount   int                    `json:"retryCount"`
}

// Status constants
const (
	StatusInactive = "inactive"
	StatusActive   = "active"
	StatusPaused   = "paused"
	StatusError    = "error"
)

// Execution status constants
const (
	ExecutionPending   = "pending"
	ExecutionRunning   = "running"
	ExecutionCompleted = "completed"
	ExecutionFailed    = "failed"
	ExecutionCancelled = "cancelled"
)

// Node types
const (
	NodeTypeTrigger     = "trigger"
	NodeTypeAction      = "action"
	NodeTypeCondition   = "condition"
	NodeTypeLoop        = "loop"
	NodeTypeMerge       = "merge"
	NodeTypeSplit       = "split"
	NodeTypeWebhook     = "webhook"
	NodeTypeHTTPRequest = "http-request"
	NodeTypeDatabase    = "database"
	NodeTypeCode        = "code"
	NodeTypeEmail       = "email"
	NodeTypeSlack       = "slack"
)

// NewWorkflow creates a new workflow
func NewWorkflow(name, description, userID string) *Workflow {
	return &Workflow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		UserID:      userID,
		Status:      StatusInactive,
		Version:     1,
		Nodes:       []Node{},
		Connections: []Connection{},
		Settings: Settings{
			Timeout:        300,
			RetryOnFailure: false,
			MaxRetries:     3,
			Timezone:       "UTC",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Validate validates the workflow structure
func (w *Workflow) Validate() error {
	// Check if workflow has at least one trigger node
	hasTrigger := false
	nodeMap := make(map[string]Node)
	
	for _, node := range w.Nodes {
		nodeMap[node.ID] = node
		if node.Type == NodeTypeTrigger {
			hasTrigger = true
		}
	}
	
	if !hasTrigger {
		return errors.New("workflow must have at least one trigger node")
	}
	
	// Validate connections
	for _, conn := range w.Connections {
		if _, ok := nodeMap[conn.Source]; !ok {
			return errors.New("invalid connection: source node not found")
		}
		if _, ok := nodeMap[conn.Target]; !ok {
			return errors.New("invalid connection: target node not found")
		}
	}
	
	// Check for cycles (simplified check)
	if w.hasCycle() {
		return errors.New("workflow contains a cycle")
	}
	
	return nil
}

// hasCycle checks if the workflow has any cycles
func (w *Workflow) hasCycle() bool {
	// Build adjacency list
	graph := make(map[string][]string)
	for _, conn := range w.Connections {
		graph[conn.Source] = append(graph[conn.Source], conn.Target)
	}
	
	// Track visited and in-progress nodes
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)
	
	// DFS to detect cycle
	var hasCycleDFS func(node string) bool
	hasCycleDFS = func(node string) bool {
		visited[node] = true
		inProgress[node] = true
		
		for _, neighbor := range graph[node] {
			if !visited[neighbor] {
				if hasCycleDFS(neighbor) {
					return true
				}
			} else if inProgress[neighbor] {
				return true
			}
		}
		
		inProgress[node] = false
		return false
	}
	
	// Check all nodes
	for _, node := range w.Nodes {
		if !visited[node.ID] {
			if hasCycleDFS(node.ID) {
				return true
			}
		}
	}
	
	return false
}

// Activate activates the workflow
func (w *Workflow) Activate() error {
	if err := w.Validate(); err != nil {
		return err
	}
	
	w.Status = StatusActive
	w.IsActive = true
	w.UpdatedAt = time.Now()
	return nil
}

// Deactivate deactivates the workflow
func (w *Workflow) Deactivate() {
	w.Status = StatusInactive
	w.IsActive = false
	w.UpdatedAt = time.Now()
}

// Clone creates a copy of the workflow
func (w *Workflow) Clone(newName string) *Workflow {
	clone := &Workflow{
		ID:          uuid.New().String(),
		Name:        newName,
		Description: w.Description,
		UserID:      w.UserID,
		TeamID:      w.TeamID,
		Nodes:       make([]Node, len(w.Nodes)),
		Connections: make([]Connection, len(w.Connections)),
		Settings:    w.Settings,
		Status:      StatusInactive,
		IsActive:    false,
		Version:     1,
		Tags:        append([]string{}, w.Tags...),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	copy(clone.Nodes, w.Nodes)
	copy(clone.Connections, w.Connections)
	
	return clone
}

// ToJSON converts workflow to JSON
func (w *Workflow) ToJSON() (string, error) {
	data, err := json.Marshal(w)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Request types for workflow operations
type CreateWorkflowRequest struct {
	UserID      string                 `json:"-"`
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Nodes       []Node                 `json:"nodes"`
	Connections []Connection           `json:"connections"`
	Settings    map[string]interface{} `json:"settings"`
	Tags        []string               `json:"tags"`
}

type UpdateWorkflowRequest struct {
	WorkflowID  string                 `json:"-"`
	UserID      string                 `json:"-"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Nodes       []Node                 `json:"nodes"`
	Connections []Connection           `json:"connections"`
	Settings    map[string]interface{} `json:"settings"`
	Tags        []string               `json:"tags"`
	Version     int                    `json:"version"`
}

type CreateVersionRequest struct {
	Message string `json:"message"`
	Changes string `json:"changes"`
}

type CreateTemplateRequest struct {
	CreatorID   string                 `json:"-"`
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Icon        string                 `json:"icon"`
	Workflow    Workflow               `json:"workflow"`
	Tags        []string               `json:"tags"`
}
