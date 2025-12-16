package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/workflow"
)

type TriggerManager interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	CreateTrigger(ctx context.Context, workflowID string, config map[string]interface{}) (*workflow.WorkflowTrigger, error)
	GetTrigger(ctx context.Context, triggerID string) (*workflow.WorkflowTrigger, error)
	ListTriggers(ctx context.Context, workflowID string) ([]*workflow.WorkflowTrigger, error)
	UpdateTrigger(ctx context.Context, triggerID string, updates map[string]interface{}) (*workflow.WorkflowTrigger, error)
	DeleteTrigger(ctx context.Context, triggerID string) error
	ActivateTrigger(ctx context.Context, triggerID string) error
	DeactivateTrigger(ctx context.Context, triggerID string) error
	TestTrigger(ctx context.Context, triggerID string, testData map[string]interface{}) (map[string]interface{}, error)
}
