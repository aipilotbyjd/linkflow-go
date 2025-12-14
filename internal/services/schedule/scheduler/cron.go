package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/schedule"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

type CronScheduler struct {
	cron       *cron.Cron
	repository ScheduleRepository
	eventBus   events.EventBus
	redis      *redis.Client
	logger     logger.Logger
	schedules  map[string]cron.EntryID
	mu         sync.RWMutex
	isLeader   bool
	stopCh     chan struct{}
}

type ScheduleRepository interface {
	Create(ctx context.Context, schedule *schedule.Schedule) error
	GetByID(ctx context.Context, id string) (*schedule.Schedule, error)
	GetActive(ctx context.Context) ([]*schedule.Schedule, error)
	Update(ctx context.Context, schedule *schedule.Schedule) error
	Delete(ctx context.Context, id string) error
	RecordExecution(ctx context.Context, execution *schedule.ScheduleExecution) error
}

func NewCronScheduler(repo ScheduleRepository, eventBus events.EventBus, redis *redis.Client, logger logger.Logger) *CronScheduler {
	// Create cron with seconds field
	c := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC))
	
	return &CronScheduler{
		cron:       c,
		repository: repo,
		eventBus:   eventBus,
		redis:      redis,
		logger:     logger,
		schedules:  make(map[string]cron.EntryID),
		stopCh:     make(chan struct{}),
	}
}

func (s *CronScheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting cron scheduler")
	
	// Start leader election
	go s.runLeaderElection(ctx)
	
	// Load active schedules
	if err := s.loadSchedules(ctx); err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}
	
	// Start cron
	s.cron.Start()
	
	// Start monitoring
	go s.monitorSchedules(ctx)
	
	return nil
}

func (s *CronScheduler) Stop() {
	s.logger.Info("Stopping cron scheduler")
	close(s.stopCh)
	
	// Stop cron
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *CronScheduler) AddSchedule(ctx context.Context, sched *schedule.Schedule) error {
	// Validate cron expression
	if _, err := cron.ParseStandard(sched.CronExpression); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	
	// Save to repository
	if err := s.repository.Create(ctx, sched); err != nil {
		return fmt.Errorf("failed to save schedule: %w", err)
	}
	
	// Add to cron if leader
	if s.isLeader {
		if err := s.addToCron(sched); err != nil {
			return fmt.Errorf("failed to add to cron: %w", err)
		}
	}
	
	// Publish schedule created event
	event := events.NewEventBuilder("schedule.created").
		WithAggregateID(sched.ID).
		WithAggregateType("schedule").
		WithPayload("workflowId", sched.WorkflowID).
		WithPayload("cron", sched.CronExpression).
		Build()
	
	s.eventBus.Publish(ctx, event)
	
	return nil
}

func (s *CronScheduler) UpdateSchedule(ctx context.Context, sched *schedule.Schedule) error {
	// Validate cron expression
	if _, err := cron.ParseStandard(sched.CronExpression); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	
	// Update in repository
	if err := s.repository.Update(ctx, sched); err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}
	
	// Update in cron if leader
	if s.isLeader {
		s.removeFromCron(sched.ID)
		if sched.IsActive {
			if err := s.addToCron(sched); err != nil {
				return fmt.Errorf("failed to update cron: %w", err)
			}
		}
	}
	
	// Publish schedule updated event
	event := events.NewEventBuilder("schedule.updated").
		WithAggregateID(sched.ID).
		WithAggregateType("schedule").
		Build()
	
	s.eventBus.Publish(ctx, event)
	
	return nil
}

func (s *CronScheduler) DeleteSchedule(ctx context.Context, scheduleID string) error {
	// Remove from cron
	s.removeFromCron(scheduleID)
	
	// Delete from repository
	if err := s.repository.Delete(ctx, scheduleID); err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}
	
	// Publish schedule deleted event
	event := events.NewEventBuilder("schedule.deleted").
		WithAggregateID(scheduleID).
		WithAggregateType("schedule").
		Build()
	
	s.eventBus.Publish(ctx, event)
	
	return nil
}

func (s *CronScheduler) PauseSchedule(ctx context.Context, scheduleID string) error {
	sched, err := s.repository.GetByID(ctx, scheduleID)
	if err != nil {
		return err
	}
	
	sched.IsActive = false
	sched.UpdatedAt = time.Now()
	
	if err := s.repository.Update(ctx, sched); err != nil {
		return err
	}
	
	// Remove from cron
	s.removeFromCron(scheduleID)
	
	return nil
}

func (s *CronScheduler) ResumeSchedule(ctx context.Context, scheduleID string) error {
	sched, err := s.repository.GetByID(ctx, scheduleID)
	if err != nil {
		return err
	}
	
	sched.IsActive = true
	sched.UpdatedAt = time.Now()
	
	if err := s.repository.Update(ctx, sched); err != nil {
		return err
	}
	
	// Add to cron if leader
	if s.isLeader {
		if err := s.addToCron(sched); err != nil {
			return err
		}
	}
	
	return nil
}

func (s *CronScheduler) loadSchedules(ctx context.Context) error {
	schedules, err := s.repository.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active schedules: %w", err)
	}
	
	for _, sched := range schedules {
		if s.isLeader {
			if err := s.addToCron(sched); err != nil {
				s.logger.Error("Failed to add schedule to cron",
					"scheduleId", sched.ID,
					"error", err,
				)
			}
		}
	}
	
	s.logger.Info("Loaded schedules", "count", len(schedules))
	return nil
}

func (s *CronScheduler) addToCron(sched *schedule.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Remove existing entry if any
	if entryID, exists := s.schedules[sched.ID]; exists {
		s.cron.Remove(entryID)
	}
	
	// Parse timezone
	// loc, err := time.LoadLocation(sched.Timezone)
	// if err != nil {
	// 	loc = time.UTC
	// }
	
	// Create cron job
	job := &scheduleJob{
		schedule:  sched,
		scheduler: s,
	}
	
	// Add to cron with timezone
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(sched.CronExpression)
	if err != nil {
		return fmt.Errorf("failed to parse cron expression: %w", err)
	}
	
	// Create a cron.Schedule wrapper that implements the interface
	entryID := s.cron.Schedule(schedule, job)
	
	s.schedules[sched.ID] = entryID
	
	s.logger.Info("Added schedule to cron",
		"scheduleId", sched.ID,
		"cron", sched.CronExpression,
		"timezone", sched.Timezone,
	)
	
	return nil
}

func (s *CronScheduler) removeFromCron(scheduleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if entryID, exists := s.schedules[scheduleID]; exists {
		s.cron.Remove(entryID)
		delete(s.schedules, scheduleID)
		
		s.logger.Info("Removed schedule from cron", "scheduleId", scheduleID)
	}
}

type scheduleJob struct {
	schedule  *schedule.Schedule
	scheduler *CronScheduler
}

func (j *scheduleJob) Run() {
	ctx := context.Background()
	
	j.scheduler.logger.Info("Executing scheduled workflow",
		"scheduleId", j.schedule.ID,
		"workflowId", j.schedule.WorkflowID,
	)
	
	// Record execution
	now := time.Now()
	execution := &schedule.ScheduleExecution{
		ID:          uuid.New().String(),
		ScheduleID:  j.schedule.ID,
		ScheduledAt: now,
		TriggeredAt: &now,
		Status:      "triggered",
	}
	
	if err := j.scheduler.repository.RecordExecution(ctx, execution); err != nil {
		j.scheduler.logger.Error("Failed to record execution", "error", err)
	}
	
	// Publish schedule triggered event
	event := events.NewEventBuilder("schedule.triggered").
		WithAggregateID(j.schedule.ID).
		WithAggregateType("schedule").
		WithPayload("workflowId", j.schedule.WorkflowID).
		WithPayload("executionId", execution.ID).
		WithPayload("data", j.schedule.Data).
		Build()
	
	if err := j.scheduler.eventBus.Publish(ctx, event); err != nil {
		j.scheduler.logger.Error("Failed to publish schedule triggered event", "error", err)
		execution.Status = "failed"
		execution.ErrorMessage = err.Error()
	} else {
		execution.Status = "success"
	}
	
	// Update execution status
	j.scheduler.repository.RecordExecution(ctx, execution)
	
	// Update last run time
	j.schedule.LastRunAt = execution.TriggeredAt
	j.schedule.NextRunAt = j.getNextRunTime()
	j.scheduler.repository.Update(ctx, j.schedule)
}

func (j *scheduleJob) getNextRunTime() *time.Time {
	entries := j.scheduler.cron.Entries()
	for _, entry := range entries {
		if entryID, exists := j.scheduler.schedules[j.schedule.ID]; exists && entry.ID == entryID {
			next := entry.Next
			return &next
		}
	}
	return nil
}

func (s *CronScheduler) runLeaderElection(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.tryBecomeLeader(ctx)
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *CronScheduler) tryBecomeLeader(ctx context.Context) {
	// Simple Redis-based leader election
	key := "scheduler:leader"
	value := uuid.New().String()
	
	// Try to acquire lock
	ok, err := s.redis.SetNX(ctx, key, value, 10*time.Second).Result()
	if err != nil {
		s.logger.Error("Failed to acquire leader lock", "error", err)
		return
	}
	
	if ok && !s.isLeader {
		s.isLeader = true
		s.logger.Info("Became leader")
		
		// Load schedules
		if err := s.loadSchedules(ctx); err != nil {
			s.logger.Error("Failed to load schedules", "error", err)
		}
	} else if !ok && s.isLeader {
		// Lost leadership
		s.isLeader = false
		s.logger.Info("Lost leadership")
		
		// Clear all schedules from cron
		s.mu.Lock()
		for id := range s.schedules {
			s.removeFromCron(id)
		}
		s.mu.Unlock()
	}
}

func (s *CronScheduler) monitorSchedules(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.checkMisfires(ctx)
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *CronScheduler) checkMisfires(ctx context.Context) {
	if !s.isLeader {
		return
	}
	
	schedules, err := s.repository.GetActive(ctx)
	if err != nil {
		s.logger.Error("Failed to get active schedules", "error", err)
		return
	}
	
	now := time.Now()
	for _, sched := range schedules {
		if sched.NextRunAt != nil && sched.NextRunAt.Before(now) {
			// Misfire detected
			s.logger.Warn("Misfire detected",
				"scheduleId", sched.ID,
				"expectedTime", sched.NextRunAt,
			)
			
			// Handle based on misfire policy
			switch sched.MisfirePolicy {
			case "run_once":
				// Run immediately
				job := &scheduleJob{schedule: sched, scheduler: s}
				go job.Run()
			case "skip":
				// Skip and update next run time
				sched.NextRunAt = s.calculateNextRunTime(sched)
				s.repository.Update(ctx, sched)
			case "run_all":
				// Run all missed executions
				// This would calculate and run all missed times
			}
		}
	}
}

func (s *CronScheduler) calculateNextRunTime(sched *schedule.Schedule) *time.Time {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(sched.CronExpression)
	if err != nil {
		return nil
	}
	
	next := schedule.Next(time.Now())
	return &next
}
