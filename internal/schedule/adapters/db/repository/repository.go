package repository

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/schedule"
	"github.com/linkflow-go/pkg/database"
)

type ScheduleRepository struct {
	db *database.DB
}

func NewScheduleRepository(db *database.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

func (r *ScheduleRepository) Create(ctx context.Context, sched *schedule.Schedule) error {
	return r.db.WithContext(ctx).Create(sched).Error
}

func (r *ScheduleRepository) Update(ctx context.Context, sched *schedule.Schedule) error {
	return r.db.WithContext(ctx).Save(sched).Error
}

func (r *ScheduleRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&schedule.Schedule{}, "id = ?", id).Error
}

func (r *ScheduleRepository) FindByID(ctx context.Context, id string) (*schedule.Schedule, error) {
	var sched schedule.Schedule
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&sched).Error
	return &sched, err
}

func (r *ScheduleRepository) FindByWorkflowID(ctx context.Context, workflowID string) ([]*schedule.Schedule, error) {
	var schedules []*schedule.Schedule
	err := r.db.WithContext(ctx).Where("workflow_id = ?", workflowID).Find(&schedules).Error
	return schedules, err
}

func (r *ScheduleRepository) FindByUserID(ctx context.Context, userID string) ([]*schedule.Schedule, error) {
	var schedules []*schedule.Schedule
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&schedules).Error
	return schedules, err
}

func (r *ScheduleRepository) FindActive(ctx context.Context) ([]*schedule.Schedule, error) {
	var schedules []*schedule.Schedule
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&schedules).Error
	return schedules, err
}

func (r *ScheduleRepository) FindDue(ctx context.Context) ([]*schedule.Schedule, error) {
	var schedules []*schedule.Schedule
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND next_run_at <= NOW()", true).
		Find(&schedules).Error
	return schedules, err
}

func (r *ScheduleRepository) GetAll(ctx context.Context) ([]*schedule.Schedule, error) {
	var schedules []*schedule.Schedule
	err := r.db.WithContext(ctx).Find(&schedules).Error
	return schedules, err
}

// GetByID is an alias for FindByID to satisfy the scheduler interface
func (r *ScheduleRepository) GetByID(ctx context.Context, id string) (*schedule.Schedule, error) {
	return r.FindByID(ctx, id)
}

// GetActive is an alias for FindActive to satisfy the scheduler interface
func (r *ScheduleRepository) GetActive(ctx context.Context) ([]*schedule.Schedule, error) {
	return r.FindActive(ctx)
}

func (r *ScheduleRepository) RecordExecution(ctx context.Context, execution *schedule.ScheduleExecution) error {
	// Record the execution in the database
	return r.db.WithContext(ctx).Create(execution).Error
}
