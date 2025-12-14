package execution

import (
	"time"

	"github.com/google/uuid"
)

// Execution represents a workflow execution instance
type Execution struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	WorkflowID    string                 `json:"workflowId" gorm:"not null;index"`
	WorkflowName  string                 `json:"workflowName"`
	Version       int                    `json:"version"`
	Status        Status                 `json:"status" gorm:"default:'pending'"`
	Priority      Priority               `json:"priority" gorm:"default:'normal'"`
	Mode          Mode                   `json:"mode" gorm:"default:'production'"`
	StartedAt     *time.Time             `json:"startedAt"`
	FinishedAt    *time.Time             `json:"finishedAt"`
	ExecutionTime int64                  `json:"executionTime"` // in milliseconds
	InputData     map[string]interface{} `json:"inputData" gorm:"serializer:json"`
	OutputData    map[string]interface{} `json:"outputData" gorm:"serializer:json"`
	Error         string                 `json:"error"`
	ErrorCode     string                 `json:"errorCode"`
	RetryCount    int                    `json:"retryCount" gorm:"default:0"`
	RetryOf       string                 `json:"retryOf"` // ID of original execution if this is a retry
	TriggeredBy   TriggerType            `json:"triggeredBy"`
	TriggerID     string                 `json:"triggerId"` // webhook/schedule/manual ID
	CreatedBy     string                 `json:"createdBy" gorm:"index"`
	CreatedAt     time.Time              `json:"createdAt"`
}

// TableName specifies the table name for GORM
func (Execution) TableName() string {
	return "execution.workflow_executions"
}

// NodeExecution represents the execution of a single node
type NodeExecution struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	ExecutionID   string                 `json:"executionId" gorm:"not null;index"`
	NodeID        string                 `json:"nodeId" gorm:"not null"`
	NodeName      string                 `json:"nodeName"`
	NodeType      string                 `json:"nodeType"`
	Status        Status                 `json:"status"`
	StartedAt     *time.Time             `json:"startedAt"`
	FinishedAt    *time.Time             `json:"finishedAt"`
	ExecutionTime int64                  `json:"executionTime"` // in milliseconds
	InputData     map[string]interface{} `json:"inputData" gorm:"serializer:json"`
	OutputData    map[string]interface{} `json:"outputData" gorm:"serializer:json"`
	Error         string                 `json:"error"`
	RetryCount    int                    `json:"retryCount" gorm:"default:0"`
	Metadata      map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	CreatedAt     time.Time              `json:"createdAt"`
}

// TableName specifies the table name for GORM
func (NodeExecution) TableName() string {
	return "execution.node_executions"
}

// Status represents execution status
type Status string

const (
	StatusPending   Status = "pending"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusTimeout   Status = "timeout"
)

// Priority represents execution priority
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
)

// Mode represents execution mode
type Mode string

const (
	ModeProduction Mode = "production"
	ModeTest       Mode = "test"
	ModeDebug      Mode = "debug"
)

// TriggerType represents what triggered the execution
type TriggerType string

const (
	TriggerManual   TriggerType = "manual"
	TriggerWebhook  TriggerType = "webhook"
	TriggerSchedule TriggerType = "schedule"
	TriggerAPI      TriggerType = "api"
	TriggerRetry    TriggerType = "retry"
	TriggerWorkflow TriggerType = "workflow" // triggered by another workflow
)

// NewExecution creates a new execution
func NewExecution(workflowID string, triggeredBy TriggerType, createdBy string) *Execution {
	return &Execution{
		ID:          uuid.New().String(),
		WorkflowID:  workflowID,
		Status:      StatusPending,
		Priority:    PriorityNormal,
		Mode:        ModeProduction,
		TriggeredBy: triggeredBy,
		CreatedBy:   createdBy,
		InputData:   make(map[string]interface{}),
		OutputData:  make(map[string]interface{}),
		CreatedAt:   time.Now(),
	}
}

// Start marks the execution as started
func (e *Execution) Start() {
	now := time.Now()
	e.Status = StatusRunning
	e.StartedAt = &now
}

// Complete marks the execution as completed
func (e *Execution) Complete(output map[string]interface{}) {
	now := time.Now()
	e.Status = StatusCompleted
	e.FinishedAt = &now
	e.OutputData = output
	if e.StartedAt != nil {
		e.ExecutionTime = now.Sub(*e.StartedAt).Milliseconds()
	}
}

// Fail marks the execution as failed
func (e *Execution) Fail(err string, errorCode string) {
	now := time.Now()
	e.Status = StatusFailed
	e.FinishedAt = &now
	e.Error = err
	e.ErrorCode = errorCode
	if e.StartedAt != nil {
		e.ExecutionTime = now.Sub(*e.StartedAt).Milliseconds()
	}
}

// Cancel marks the execution as cancelled
func (e *Execution) Cancel() {
	now := time.Now()
	e.Status = StatusCancelled
	e.FinishedAt = &now
	if e.StartedAt != nil {
		e.ExecutionTime = now.Sub(*e.StartedAt).Milliseconds()
	}
}

// IsTerminal checks if the execution is in a terminal state
func (e *Execution) IsTerminal() bool {
	return e.Status == StatusCompleted ||
		e.Status == StatusFailed ||
		e.Status == StatusCancelled ||
		e.Status == StatusTimeout
}

// CanRetry checks if the execution can be retried
func (e *Execution) CanRetry(maxRetries int) bool {
	return e.Status == StatusFailed && e.RetryCount < maxRetries
}

// NewNodeExecution creates a new node execution
func NewNodeExecution(executionID, nodeID, nodeName, nodeType string) *NodeExecution {
	return &NodeExecution{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		NodeID:      nodeID,
		NodeName:    nodeName,
		NodeType:    nodeType,
		Status:      StatusPending,
		InputData:   make(map[string]interface{}),
		OutputData:  make(map[string]interface{}),
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now(),
	}
}

// Start marks the node execution as started
func (n *NodeExecution) Start(input map[string]interface{}) {
	now := time.Now()
	n.Status = StatusRunning
	n.StartedAt = &now
	n.InputData = input
}

// Complete marks the node execution as completed
func (n *NodeExecution) Complete(output map[string]interface{}) {
	now := time.Now()
	n.Status = StatusCompleted
	n.FinishedAt = &now
	n.OutputData = output
	if n.StartedAt != nil {
		n.ExecutionTime = now.Sub(*n.StartedAt).Milliseconds()
	}
}

// Fail marks the node execution as failed
func (n *NodeExecution) Fail(err string) {
	now := time.Now()
	n.Status = StatusFailed
	n.FinishedAt = &now
	n.Error = err
	if n.StartedAt != nil {
		n.ExecutionTime = now.Sub(*n.StartedAt).Milliseconds()
	}
}
