package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/schedule"
)

type ScheduleRepository interface {
	Create(ctx context.Context, schedule *schedule.Schedule) error
	FindByID(ctx context.Context, id string) (*schedule.Schedule, error)
	FindByWorkflowID(ctx context.Context, workflowID string) ([]*schedule.Schedule, error)
	FindByUserID(ctx context.Context, userID string) ([]*schedule.Schedule, error)
	FindActive(ctx context.Context) ([]*schedule.Schedule, error)
	FindDue(ctx context.Context) ([]*schedule.Schedule, error)
	GetAll(ctx context.Context) ([]*schedule.Schedule, error)

	GetByID(ctx context.Context, id string) (*schedule.Schedule, error)
	GetActive(ctx context.Context) ([]*schedule.Schedule, error)
	Update(ctx context.Context, schedule *schedule.Schedule) error
	Delete(ctx context.Context, id string) error
	RecordExecution(ctx context.Context, execution *schedule.ScheduleExecution) error
}
