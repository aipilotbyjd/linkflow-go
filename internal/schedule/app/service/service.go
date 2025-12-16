package service

import (
	"context"
	"errors"

	"github.com/linkflow-go/internal/schedule/ports"
	"github.com/linkflow-go/pkg/contracts/schedule"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// UpdateScheduleRequest represents a schedule update request
type UpdateScheduleRequest struct {
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	CronExpression string                 `json:"cronExpression"`
	Timezone       string                 `json:"timezone"`
	Data           map[string]interface{} `json:"data"`
}

type ScheduleService struct {
	repo     ports.ScheduleRepository
	eventBus events.EventBus
	logger   logger.Logger
}

func NewScheduleService(
	repo ports.ScheduleRepository,
	eventBus events.EventBus,
	log logger.Logger,
) *ScheduleService {
	return &ScheduleService{
		repo:     repo,
		eventBus: eventBus,
		logger:   log,
	}
}

// ListSchedules returns all schedules for a user, optionally filtered by workflow
func (s *ScheduleService) ListSchedules(ctx context.Context, userID, workflowID string) ([]*schedule.Schedule, error) {
	if workflowID != "" {
		return s.repo.FindByWorkflowID(ctx, workflowID)
	}
	return s.repo.FindByUserID(ctx, userID)
}

// GetSchedule returns a schedule by ID
func (s *ScheduleService) GetSchedule(ctx context.Context, id string) (*schedule.Schedule, error) {
	return s.repo.FindByID(ctx, id)
}

// CreateSchedule creates a new schedule
func (s *ScheduleService) CreateSchedule(ctx context.Context, sched *schedule.Schedule) error {
	if err := sched.Validate(); err != nil {
		return err
	}

	if err := s.repo.Create(ctx, sched); err != nil {
		return err
	}

	// Publish event
	s.eventBus.Publish(ctx, events.Event{
		Type:        "schedule.created",
		AggregateID: sched.ID,
		Payload: map[string]interface{}{
			"scheduleId": sched.ID,
			"workflowId": sched.WorkflowID,
			"cron":       sched.CronExpression,
		},
	})

	s.logger.Info("Schedule created", "id", sched.ID, "workflowId", sched.WorkflowID)
	return nil
}

// UpdateSchedule updates a schedule
func (s *ScheduleService) UpdateSchedule(ctx context.Context, id string, req *UpdateScheduleRequest) (*schedule.Schedule, error) {
	sched, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		sched.Name = req.Name
	}
	if req.Description != "" {
		sched.Description = req.Description
	}
	if req.CronExpression != "" {
		sched.CronExpression = req.CronExpression
	}
	if req.Timezone != "" {
		sched.Timezone = req.Timezone
	}
	if req.Data != nil {
		sched.Data = req.Data
	}

	if err := sched.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, sched); err != nil {
		return nil, err
	}

	s.logger.Info("Schedule updated", "id", id)
	return sched, nil
}

// DeleteSchedule deletes a schedule
func (s *ScheduleService) DeleteSchedule(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.eventBus.Publish(ctx, events.Event{
		Type:        "schedule.deleted",
		AggregateID: id,
	})

	s.logger.Info("Schedule deleted", "id", id)
	return nil
}

// PauseSchedule pauses a schedule
func (s *ScheduleService) PauseSchedule(ctx context.Context, id string) error {
	sched, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	sched.IsActive = false
	if err := s.repo.Update(ctx, sched); err != nil {
		return err
	}

	s.eventBus.Publish(ctx, events.Event{
		Type:        "schedule.paused",
		AggregateID: id,
	})

	s.logger.Info("Schedule paused", "id", id)
	return nil
}

// ResumeSchedule resumes a schedule
func (s *ScheduleService) ResumeSchedule(ctx context.Context, id string) error {
	sched, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	sched.IsActive = true
	if err := s.repo.Update(ctx, sched); err != nil {
		return err
	}

	s.eventBus.Publish(ctx, events.Event{
		Type:        "schedule.resumed",
		AggregateID: id,
	})

	s.logger.Info("Schedule resumed", "id", id)
	return nil
}

// TriggerSchedule manually triggers a schedule execution
func (s *ScheduleService) TriggerSchedule(ctx context.Context, id string) (string, error) {
	sched, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}

	if !sched.IsActive {
		return "", errors.New("schedule is not active")
	}

	// Publish event to trigger execution
	s.eventBus.Publish(ctx, events.Event{
		Type:        "schedule.triggered",
		AggregateID: id,
		Payload: map[string]interface{}{
			"scheduleId": sched.ID,
			"workflowId": sched.WorkflowID,
			"data":       sched.Data,
			"manual":     true,
		},
	})

	s.logger.Info("Schedule manually triggered", "id", id, "workflowId", sched.WorkflowID)

	// Return a placeholder execution ID - actual ID would come from execution service
	return "pending", nil
}

// GetActiveSchedules returns all active schedules
func (s *ScheduleService) GetActiveSchedules(ctx context.Context) ([]*schedule.Schedule, error) {
	return s.repo.FindActive(ctx)
}

// GetDueSchedules returns schedules that are due to run
func (s *ScheduleService) GetDueSchedules(ctx context.Context) ([]*schedule.Schedule, error) {
	return s.repo.FindDue(ctx)
}
