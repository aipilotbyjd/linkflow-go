package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/workflow"
)

type WorkflowRepository interface {
	Ping(ctx context.Context) error

	CreateWorkflow(ctx context.Context, w *workflow.Workflow) error
	CreateWithVersion(ctx context.Context, w *workflow.Workflow) error
	GetWorkflow(ctx context.Context, workflowID, userID string) (*workflow.Workflow, error)
	UpdateWorkflow(ctx context.Context, w *workflow.Workflow) error
	UpdateWithVersion(ctx context.Context, w *workflow.Workflow, changeNote string) error
	DeleteWorkflow(ctx context.Context, workflowID, userID string) error

	ListWorkflows(ctx context.Context, opts ListWorkflowsOptions) ([]*workflow.Workflow, int64, error)

	ListVersions(ctx context.Context, workflowID string) ([]*workflow.WorkflowVersion, error)
	GetVersion(ctx context.Context, workflowID string, version int) (*workflow.WorkflowVersion, error)
	RestoreVersion(ctx context.Context, workflowID string, version int, userID string) error

	// Permissions
	ListWorkflowPermissions(ctx context.Context, workflowID string) ([]map[string]interface{}, error)
	CreateWorkflowPermission(ctx context.Context, permission map[string]interface{}) error
	DeleteWorkflowPermission(ctx context.Context, workflowID, userID string) (int64, error)

	// Categories
	CreateCategory(ctx context.Context, category map[string]interface{}) error

	// Stats & Executions
	GetWorkflowStats(ctx context.Context, workflowID string) (WorkflowStats, error)
	ListWorkflowExecutions(ctx context.Context, workflowID string, offset, limit int) ([]workflow.WorkflowExecution, int64, error)
	GetLatestWorkflowExecution(ctx context.Context, workflowID string) (*workflow.WorkflowExecution, error)
	GetPopularTags(ctx context.Context, limit int) ([]string, error)

	// Variables
	SaveWorkflowVariable(ctx context.Context, variable *workflow.WorkflowVariable) error
	GetWorkflowVariable(ctx context.Context, workflowID, key string) (*workflow.WorkflowVariable, error)
	ListWorkflowVariables(ctx context.Context, workflowID string) ([]*workflow.WorkflowVariable, error)
	DeleteWorkflowVariable(ctx context.Context, workflowID, key string) (int64, error)

	// Environments
	CountEnvironments(ctx context.Context, workflowID string) (int64, error)
	CreateEnvironment(ctx context.Context, env *workflow.Environment) error
	GetEnvironment(ctx context.Context, workflowID, envID string) (*workflow.Environment, error)
	ListEnvironments(ctx context.Context, workflowID string) ([]*workflow.Environment, error)
	UpdateEnvironment(ctx context.Context, workflowID, envID string, updates map[string]interface{}) (int64, error)
	DeleteEnvironment(ctx context.Context, env *workflow.Environment) error
	SetDefaultEnvironment(ctx context.Context, workflowID, envID string) (int64, error)
}

type WorkflowStats struct {
	TotalExecutions   int64   `json:"total_executions"`
	SuccessfulRuns    int64   `json:"successful_runs"`
	FailedRuns        int64   `json:"failed_runs"`
	AvgExecutionTime  float64 `json:"avg_execution_time_ms"`
	LastExecutionTime *string `json:"last_execution_time"`
}

type ListWorkflowsOptions struct {
	UserID   string
	TeamID   string
	Status   string
	IsActive *bool
	Tags     []string
	Search   string
	Page     int
	Limit    int
	SortBy   string
	SortDesc bool
}
