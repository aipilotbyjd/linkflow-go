package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/workflow"
)

type ExecutionRepository interface {
	Create(ctx context.Context, execution *workflow.WorkflowExecution) error
	Update(ctx context.Context, execution *workflow.WorkflowExecution) error
	GetByID(ctx context.Context, id string) (*workflow.WorkflowExecution, error)
	GetWorkflow(ctx context.Context, workflowID string) (*workflow.Workflow, error)
	CreateNodeExecution(ctx context.Context, nodeExec *workflow.NodeExecution) error
	UpdateNodeExecution(ctx context.Context, nodeExec *workflow.NodeExecution) error
}
