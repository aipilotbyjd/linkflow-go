package workflow

import (
	"time"
)

// ExecutionStatus represents the status of an execution
type ExecutionStatus string

const (
	ExecutionPending    ExecutionStatus = "pending"
	ExecutionQueued     ExecutionStatus = "queued"
	ExecutionRunning    ExecutionStatus = "running"
	ExecutionPaused     ExecutionStatus = "paused"
	ExecutionCompleted  ExecutionStatus = "completed"
	ExecutionFailed     ExecutionStatus = "failed"
	ExecutionCancelled  ExecutionStatus = "cancelled"
	ExecutionTimeout    ExecutionStatus = "timeout"
)

// ExecutionPriority represents the priority of an execution
type ExecutionPriority string

const (
	PriorityHigh   ExecutionPriority = "high"
	PriorityNormal ExecutionPriority = "normal"
	PriorityLow    ExecutionPriority = "low"
)

// ExecutionRequest represents a request to execute a workflow
type ExecutionRequest struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Priority    ExecutionPriority      `json:"priority"`
	InputData   map[string]interface{} `json:"input_data"`
	Metadata    map[string]string      `json:"metadata"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	RequestedBy string                 `json:"requested_by"`
	RequestedAt time.Time              `json:"requested_at"`
}

// ExecutionContext represents the runtime context of an execution
type ExecutionContext struct {
	ExecutionID string                    `json:"execution_id"`
	Variables   map[string]interface{}    `json:"variables"`
	NodeOutputs map[string]interface{}    `json:"node_outputs"`
	Errors      []ExecutionErrorDetail    `json:"errors"`
	StartTime   time.Time                 `json:"start_time"`
	Metadata    map[string]string         `json:"metadata"`
}

// ExecutionErrorDetail represents detailed error information
type ExecutionErrorDetail struct {
	NodeID    string    `json:"node_id"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	Retryable bool      `json:"retryable"`
}

// NodeExecutionStatus represents the status of a node execution
type NodeExecutionStatus string

const (
	NodeExecutionPending   NodeExecutionStatus = "pending"
	NodeExecutionRunning   NodeExecutionStatus = "running"
	NodeExecutionCompleted NodeExecutionStatus = "completed"
	NodeExecutionFailed    NodeExecutionStatus = "failed"
	NodeExecutionSkipped   NodeExecutionStatus = "skipped"
	NodeExecutionCancelled NodeExecutionStatus = "cancelled"
)

// ParallelExecutionGroup represents a group of nodes that can execute in parallel
type ParallelExecutionGroup struct {
	ID      string   `json:"id"`
	NodeIDs []string `json:"node_ids"`
	MaxConcurrency int `json:"max_concurrency"`
}

// ExecutionMetrics represents metrics for an execution
type ExecutionMetrics struct {
	ExecutionID      string        `json:"execution_id"`
	TotalNodes       int           `json:"total_nodes"`
	CompletedNodes   int           `json:"completed_nodes"`
	FailedNodes      int           `json:"failed_nodes"`
	SkippedNodes     int           `json:"skipped_nodes"`
	TotalDuration    time.Duration `json:"total_duration"`
	NodeDurations    map[string]time.Duration `json:"node_durations"`
	MemoryUsage      int64         `json:"memory_usage_bytes"`
	CPUUsage         float64       `json:"cpu_usage_percent"`
}

// ExecutionCheckpoint represents a checkpoint in the execution
type ExecutionCheckpoint struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id"`
	State       map[string]interface{} `json:"state"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     int                    `json:"version"`
}

// RetryPolicy defines retry behavior for executions
type RetryPolicy struct {
	MaxAttempts      int           `json:"max_attempts"`
	InitialInterval  time.Duration `json:"initial_interval"`
	MaxInterval      time.Duration `json:"max_interval"`
	BackoffMultiplier float64      `json:"backoff_multiplier"`
	RetryableErrors  []string      `json:"retryable_errors"`
}

// ExecutionSchedule represents a scheduled execution
type ExecutionSchedule struct {
	ID               string            `json:"id"`
	WorkflowID       string            `json:"workflow_id"`
	CronExpression   string            `json:"cron_expression,omitempty"`
	Interval         *time.Duration    `json:"interval,omitempty"`
	NextRunTime      time.Time         `json:"next_run_time"`
	LastRunTime      *time.Time        `json:"last_run_time,omitempty"`
	InputData        map[string]interface{} `json:"input_data"`
	Enabled          bool              `json:"enabled"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// ExecutionLog represents a log entry for an execution
type ExecutionLog struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id,omitempty"`
	Level       string                 `json:"level"`
	Message     string                 `json:"message"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ExecutionTrace represents a trace span for distributed tracing
type ExecutionTrace struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id,omitempty"`
	SpanID      string                 `json:"span_id"`
	ParentSpanID string                `json:"parent_span_id,omitempty"`
	TraceID     string                 `json:"trace_id"`
	Operation   string                 `json:"operation"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Tags        map[string]interface{} `json:"tags,omitempty"`
	Status      string                 `json:"status"`
}

// ExecutionEvent represents an event that occurred during execution
type ExecutionEvent struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id,omitempty"`
	EventType   string                 `json:"event_type"`
	EventData   map[string]interface{} `json:"event_data"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ExecutionDependency represents dependencies between node executions
type ExecutionDependency struct {
	NodeID           string   `json:"node_id"`
	DependsOn        []string `json:"depends_on"`
	WaitForAll       bool     `json:"wait_for_all"`
	ContinueOnFail   bool     `json:"continue_on_fail"`
}

// ExecutionGraph represents the execution graph for a workflow
type ExecutionGraph struct {
	Nodes         map[string]*Node              `json:"nodes"`
	Dependencies  map[string]*ExecutionDependency `json:"dependencies"`
	ParallelGroups []ParallelExecutionGroup     `json:"parallel_groups"`
}

// BuildExecutionGraph builds an execution graph from a workflow
func BuildExecutionGraph(workflow *Workflow) *ExecutionGraph {
	graph := &ExecutionGraph{
		Nodes:          make(map[string]*Node),
		Dependencies:   make(map[string]*ExecutionDependency),
		ParallelGroups: []ParallelExecutionGroup{},
	}
	
	// Add nodes to graph
	for i := range workflow.Nodes {
		node := &workflow.Nodes[i]
		graph.Nodes[node.ID] = node
	}
	
	// Build dependencies from connections
	for _, conn := range workflow.Connections {
		if dep, exists := graph.Dependencies[conn.Target]; exists {
			dep.DependsOn = append(dep.DependsOn, conn.Source)
		} else {
			graph.Dependencies[conn.Target] = &ExecutionDependency{
				NodeID:    conn.Target,
				DependsOn: []string{conn.Source},
				WaitForAll: true,
			}
		}
	}
	
	// Identify parallel execution groups
	graph.identifyParallelGroups()
	
	return graph
}

// identifyParallelGroups identifies nodes that can execute in parallel
func (g *ExecutionGraph) identifyParallelGroups() {
	// Find nodes with the same dependencies
	depGroups := make(map[string][]string)
	
	for nodeID, dep := range g.Dependencies {
		if len(dep.DependsOn) > 0 {
			// Create a key from sorted dependencies
			key := createDependencyKey(dep.DependsOn)
			depGroups[key] = append(depGroups[key], nodeID)
		}
	}
	
	// Create parallel groups from nodes with same dependencies
	for _, nodes := range depGroups {
		if len(nodes) > 1 {
			g.ParallelGroups = append(g.ParallelGroups, ParallelExecutionGroup{
				ID:             generateGroupID(),
				NodeIDs:        nodes,
				MaxConcurrency: len(nodes), // Default to unlimited concurrency
			})
		}
	}
}

// Helper functions
func createDependencyKey(deps []string) string {
	// Simple concatenation for demo - in production, use proper sorting
	key := ""
	for _, dep := range deps {
		key += dep + ","
	}
	return key
}

func generateGroupID() string {
	return "group_" + time.Now().Format("20060102150405")
}
