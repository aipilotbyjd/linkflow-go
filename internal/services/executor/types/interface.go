package types

import (
	"context"
	"time"
)

// Node represents a workflow node
type Node struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Config     interface{}            `json:"config"`
	Parameters map[string]interface{} `json:"parameters"`
	Disabled   bool                   `json:"disabled"`
	RetryCount int                    `json:"retryCount"`
	Timeout    int                    `json:"timeout"`
}

// NodeExecutor is the interface that all node executors must implement
type NodeExecutor interface {
	// Execute runs the node with the given input
	Execute(ctx context.Context, node Node, input map[string]interface{}) (output map[string]interface{}, err error)
	
	// ValidateInput validates the input for the node
	ValidateInput(node Node, input map[string]interface{}) error
	
	// GetTimeout returns the timeout duration for the node
	GetTimeout() time.Duration
}

// NodeExecutorFactory creates node executors based on type
type NodeExecutorFactory interface {
	// CreateExecutor creates an executor for the given node type
	CreateExecutor(nodeType string) (NodeExecutor, error)
	
	// RegisterExecutor registers a custom executor for a node type
	RegisterExecutor(nodeType string, executor NodeExecutor) error
	
	// GetSupportedTypes returns a list of supported node types
	GetSupportedTypes() []string
}

// ExecutionContext provides context for node execution
type ExecutionContext struct {
	ExecutionID string                 `json:"executionId"`
	WorkflowID  string                 `json:"workflowId"`
	NodeID      string                 `json:"nodeId"`
	Variables   map[string]interface{} `json:"variables"`
	Secrets     map[string]string      `json:"secrets"`
	StartTime   time.Time              `json:"startTime"`
}

// ExecutionResult represents the result of node execution
type ExecutionResult struct {
	Success   bool                   `json:"success"`
	Output    map[string]interface{} `json:"output"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
