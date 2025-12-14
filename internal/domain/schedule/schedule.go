package schedule

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Schedule struct {
	ID             string                 `json:"id" gorm:"primaryKey"`
	Name           string                 `json:"name" gorm:"not null"`
	Description    string                 `json:"description"`
	WorkflowID     string                 `json:"workflowId" gorm:"column:workflow_id;not null;index"`
	UserID         string                 `json:"userId" gorm:"column:user_id;not null;index"`
	TeamID         string                 `json:"teamId" gorm:"column:team_id;index"`
	CronExpression string                 `json:"cronExpression" gorm:"column:cron_expression;not null"`
	Timezone       string                 `json:"timezone" gorm:"default:'UTC'"`
	Data           map[string]interface{} `json:"data" gorm:"column:input_data;serializer:json"`
	IsActive       bool                   `json:"isActive" gorm:"column:is_active;default:true"`
	StartDate      *time.Time             `json:"startDate" gorm:"column:start_date"`
	EndDate        *time.Time             `json:"endDate" gorm:"column:end_date"`
	LastRunAt      *time.Time             `json:"lastRunAt" gorm:"column:last_run_at"`
	NextRunAt      *time.Time             `json:"nextRunAt" gorm:"column:next_run_at"`
	RunCount       int64                  `json:"runCount" gorm:"column:run_count;default:0"`
	MisfirePolicy  string                 `json:"misfirePolicy" gorm:"column:misfire_policy;default:'skip'"`
	MaxRetries     int                    `json:"maxRetries" gorm:"column:max_retries;default:3"`
	Tags           []string               `json:"tags" gorm:"type:text[];serializer:json"`
	CreatedAt      time.Time              `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt      time.Time              `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (Schedule) TableName() string {
	return "schedule.schedules"
}

type ScheduleExecution struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	ScheduleID   string     `json:"scheduleId" gorm:"column:schedule_id;not null;index"`
	ExecutionID  string     `json:"executionId" gorm:"column:execution_id"`
	ScheduledAt  time.Time  `json:"scheduledAt" gorm:"column:scheduled_at;not null"`
	TriggeredAt  *time.Time `json:"triggeredAt" gorm:"column:triggered_at"`
	Status       string     `json:"status" gorm:"default:'pending'"`
	ErrorMessage string     `json:"error" gorm:"column:error_message"`
	CreatedAt    time.Time  `json:"createdAt" gorm:"column:created_at"`
}

// TableName specifies the table name for ScheduleExecution
func (ScheduleExecution) TableName() string {
	return "schedule.schedule_executions"
}

// Misfire policies
const (
	MisfirePolicySkip    = "skip"     // Skip missed executions
	MisfirePolicyRunOnce = "run_once" // Run once immediately
	MisfirePolicyRunAll  = "run_all"  // Run all missed executions
)

// Execution status
const (
	ExecutionStatusTriggered = "triggered"
	ExecutionStatusSuccess   = "success"
	ExecutionStatusFailed    = "failed"
	ExecutionStatusSkipped   = "skipped"
)

// Predefined cron expressions
var PredefinedSchedules = map[string]string{
	"every_minute":       "0 * * * * *",
	"every_5_minutes":    "0 */5 * * * *",
	"every_15_minutes":   "0 */15 * * * *",
	"every_30_minutes":   "0 */30 * * * *",
	"every_hour":         "0 0 * * * *",
	"every_day_midnight": "0 0 0 * * *",
	"every_day_noon":     "0 0 12 * * *",
	"every_week_monday":  "0 0 0 * * 1",
	"every_month_first":  "0 0 0 1 * *",
}

// NewSchedule creates a new schedule
func NewSchedule(name, workflowID, userID, cronExpression string) *Schedule {
	return &Schedule{
		ID:             uuid.New().String(),
		Name:           name,
		WorkflowID:     workflowID,
		UserID:         userID,
		CronExpression: cronExpression,
		Timezone:       "UTC",
		IsActive:       true,
		MisfirePolicy:  MisfirePolicySkip,
		Data:           make(map[string]interface{}),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// Validate validates the schedule
func (s *Schedule) Validate() error {
	if s.Name == "" {
		return errors.New("schedule name is required")
	}
	if s.WorkflowID == "" {
		return errors.New("workflow ID is required")
	}
	if s.CronExpression == "" {
		return errors.New("cron expression is required")
	}

	// Validate cron expression format
	// This is a simplified validation - in production use a proper cron parser
	if len(s.CronExpression) < 9 { // Minimum: "* * * * *"
		return errors.New("invalid cron expression")
	}

	// Validate timezone
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return errors.New("invalid timezone")
	}

	// Validate date range
	if s.StartDate != nil && s.EndDate != nil {
		if s.StartDate.After(*s.EndDate) {
			return errors.New("start date must be before end date")
		}
	}

	// Validate misfire policy
	validPolicies := []string{MisfirePolicySkip, MisfirePolicyRunOnce, MisfirePolicyRunAll}
	valid := false
	for _, policy := range validPolicies {
		if s.MisfirePolicy == policy {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("invalid misfire policy")
	}

	return nil
}

// IsExpired checks if the schedule has expired
func (s *Schedule) IsExpired() bool {
	if s.EndDate == nil {
		return false
	}
	return time.Now().After(*s.EndDate)
}

// ShouldRun checks if the schedule should run at the given time
func (s *Schedule) ShouldRun(t time.Time) bool {
	if !s.IsActive {
		return false
	}

	if s.IsExpired() {
		return false
	}

	if s.StartDate != nil && t.Before(*s.StartDate) {
		return false
	}

	return true
}

// GetNextRunTime calculates the next run time
// This is a placeholder - actual implementation would use cron parser
func (s *Schedule) GetNextRunTime(from time.Time) *time.Time {
	// This would parse the cron expression and calculate next run
	// For now, return a simple calculation
	next := from.Add(1 * time.Hour)
	return &next
}

// RecordRun records a schedule run
func (s *Schedule) RecordRun(t time.Time) {
	s.LastRunAt = &t
	s.NextRunAt = s.GetNextRunTime(t)
	s.UpdatedAt = time.Now()
}

// GetTimezone returns the timezone location
func (s *Schedule) GetTimezone() *time.Location {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// FormatCronExpression formats the cron expression for display
func (s *Schedule) FormatCronExpression() string {
	// Check if it's a predefined schedule
	for key, expr := range PredefinedSchedules {
		if expr == s.CronExpression {
			return key
		}
	}
	return s.CronExpression
}

// ParseCronExpression parses a cron expression or predefined key
func ParseCronExpression(input string) (string, error) {
	// Check if it's a predefined schedule
	if expr, ok := PredefinedSchedules[input]; ok {
		return expr, nil
	}

	// Validate custom cron expression
	// This is simplified - use a proper cron parser in production
	if len(input) < 9 {
		return "", errors.New("invalid cron expression")
	}

	return input, nil
}
