package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Trigger types
const (
	TriggerTypeWebhook  = "webhook"
	TriggerTypeSchedule = "schedule"
	TriggerTypeEvent    = "event"
	TriggerTypeManual   = "manual"
	TriggerTypeEmail    = "email"
	TriggerTypeAPI      = "api"
)

// Trigger status
const (
	TriggerStatusActive   = "active"
	TriggerStatusInactive = "inactive"
	TriggerStatusPaused   = "paused"
)

// Trigger represents a workflow trigger
type Trigger interface {
	GetID() string
	GetType() string
	GetWorkflowID() string
	Validate() error
	GetConfig() map[string]interface{}
	ShouldFire(event interface{}) bool
	IsActive() bool
	GetStatus() string
}

// BaseTrigger contains common trigger fields
type BaseTrigger struct {
	ID         string                 `json:"id" gorm:"primaryKey"`
	WorkflowID string                 `json:"workflowId" gorm:"not null;index"`
	Type       string                 `json:"type" gorm:"not null"`
	Name       string                 `json:"name"`
	Status     string                 `json:"status" gorm:"default:'inactive'"`
	Config     map[string]interface{} `json:"config" gorm:"serializer:json"`
	CreatedAt  time.Time              `json:"createdAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
	LastFired  *time.Time             `json:"lastFired"`
	FireCount  int                    `json:"fireCount" gorm:"default:0"`
}

// WorkflowTrigger represents a trigger stored in database
type WorkflowTrigger struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	WorkflowID  string                 `json:"workflowId" gorm:"not null;index"`
	Type        string                 `json:"type" gorm:"not null"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status" gorm:"default:'inactive'"`
	Config      json.RawMessage        `json:"config" gorm:"type:jsonb"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	LastFired   *time.Time             `json:"lastFired"`
	FireCount   int64                  `json:"fireCount" gorm:"default:0"`
	ErrorCount  int64                  `json:"errorCount" gorm:"default:0"`
	LastError   string                 `json:"lastError"`
}

// GetID returns the trigger ID
func (t *BaseTrigger) GetID() string {
	return t.ID
}

// GetType returns the trigger type
func (t *BaseTrigger) GetType() string {
	return t.Type
}

// GetWorkflowID returns the workflow ID
func (t *BaseTrigger) GetWorkflowID() string {
	return t.WorkflowID
}

// IsActive checks if trigger is active
func (t *BaseTrigger) IsActive() bool {
	return t.Status == TriggerStatusActive
}

// GetStatus returns the trigger status
func (t *BaseTrigger) GetStatus() string {
	return t.Status
}

// GetConfig returns the trigger configuration
func (t *BaseTrigger) GetConfig() map[string]interface{} {
	return t.Config
}

// WebhookTrigger represents a webhook trigger
type WebhookTrigger struct {
	BaseTrigger
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Secret      string            `json:"secret"`
	ValidateSSL bool              `json:"validateSSL"`
}

// NewWebhookTrigger creates a new webhook trigger
func NewWebhookTrigger(workflowID, name, path string) *WebhookTrigger {
	return &WebhookTrigger{
		BaseTrigger: BaseTrigger{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			Type:       TriggerTypeWebhook,
			Name:       name,
			Status:     TriggerStatusInactive,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Config:     make(map[string]interface{}),
		},
		Path:   path,
		Method: "POST",
	}
}

// Validate validates the webhook trigger
func (t *WebhookTrigger) Validate() error {
	if t.Path == "" {
		return errors.New("webhook path is required")
	}
	
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
	if !validMethods[t.Method] {
		return fmt.Errorf("invalid HTTP method: %s", t.Method)
	}
	
	// Update config
	t.Config["path"] = t.Path
	t.Config["method"] = t.Method
	t.Config["secret"] = t.Secret
	
	return nil
}

// ShouldFire checks if the webhook should fire for given event
func (t *WebhookTrigger) ShouldFire(event interface{}) bool {
	if !t.IsActive() {
		return false
	}
	
	webhookEvent, ok := event.(map[string]interface{})
	if !ok {
		return false
	}
	
	// Check if path matches
	if path, ok := webhookEvent["path"].(string); ok {
		if path != t.Path {
			return false
		}
	}
	
	// Check if method matches
	if method, ok := webhookEvent["method"].(string); ok {
		if method != t.Method {
			return false
		}
	}
	
	// Validate secret if configured
	if t.Secret != "" {
		if secret, ok := webhookEvent["secret"].(string); ok {
			return secret == t.Secret
		}
		return false
	}
	
	return true
}

// ScheduleTrigger represents a cron schedule trigger
type ScheduleTrigger struct {
	BaseTrigger
	CronExpression string     `json:"cronExpression"`
	Timezone       string     `json:"timezone"`
	StartDate      *time.Time `json:"startDate"`
	EndDate        *time.Time `json:"endDate"`
}

// NewScheduleTrigger creates a new schedule trigger
func NewScheduleTrigger(workflowID, name, cronExpression string) *ScheduleTrigger {
	return &ScheduleTrigger{
		BaseTrigger: BaseTrigger{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			Type:       TriggerTypeSchedule,
			Name:       name,
			Status:     TriggerStatusInactive,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Config:     make(map[string]interface{}),
		},
		CronExpression: cronExpression,
		Timezone:       "UTC",
	}
}

// Validate validates the schedule trigger
func (t *ScheduleTrigger) Validate() error {
	if t.CronExpression == "" {
		return errors.New("cron expression is required")
	}
	
	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(t.CronExpression); err != nil {
		return fmt.Errorf("invalid cron expression: %v", err)
	}
	
	// Validate timezone
	if _, err := time.LoadLocation(t.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %v", err)
	}
	
	// Check date range
	if t.StartDate != nil && t.EndDate != nil {
		if t.StartDate.After(*t.EndDate) {
			return errors.New("start date must be before end date")
		}
	}
	
	// Update config
	t.Config["cronExpression"] = t.CronExpression
	t.Config["timezone"] = t.Timezone
	if t.StartDate != nil {
		t.Config["startDate"] = t.StartDate.Format(time.RFC3339)
	}
	if t.EndDate != nil {
		t.Config["endDate"] = t.EndDate.Format(time.RFC3339)
	}
	
	return nil
}

// ShouldFire checks if the schedule should fire for given time
func (t *ScheduleTrigger) ShouldFire(event interface{}) bool {
	if !t.IsActive() {
		return false
	}
	
	now, ok := event.(time.Time)
	if !ok {
		return false
	}
	
	// Check date range
	if t.StartDate != nil && now.Before(*t.StartDate) {
		return false
	}
	if t.EndDate != nil && now.After(*t.EndDate) {
		return false
	}
	
	// Parse cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(t.CronExpression)
	if err != nil {
		return false
	}
	
	// Check if it's time to fire
	// This is simplified - in production, you'd track last execution time
	// TODO: Use the schedule to check actual timing
	return true
}

// GetNextRunTime calculates the next run time for the schedule
func (t *ScheduleTrigger) GetNextRunTime() (*time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(t.CronExpression)
	if err != nil {
		return nil, err
	}
	
	loc, err := time.LoadLocation(t.Timezone)
	if err != nil {
		return nil, err
	}
	
	now := time.Now().In(loc)
	
	// Check date range
	if t.StartDate != nil && now.Before(*t.StartDate) {
		now = *t.StartDate
	}
	
	next := schedule.Next(now)
	
	if t.EndDate != nil && next.After(*t.EndDate) {
		return nil, errors.New("next run time is after end date")
	}
	
	return &next, nil
}

// EventTrigger represents an event-based trigger
type EventTrigger struct {
	BaseTrigger
	EventType   string                 `json:"eventType"`
	EventSource string                 `json:"eventSource"`
	Filters     map[string]interface{} `json:"filters"`
}

// NewEventTrigger creates a new event trigger
func NewEventTrigger(workflowID, name, eventType string) *EventTrigger {
	return &EventTrigger{
		BaseTrigger: BaseTrigger{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			Type:       TriggerTypeEvent,
			Name:       name,
			Status:     TriggerStatusInactive,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Config:     make(map[string]interface{}),
		},
		EventType: eventType,
		Filters:   make(map[string]interface{}),
	}
}

// Validate validates the event trigger
func (t *EventTrigger) Validate() error {
	if t.EventType == "" {
		return errors.New("event type is required")
	}
	
	// Update config
	t.Config["eventType"] = t.EventType
	t.Config["eventSource"] = t.EventSource
	t.Config["filters"] = t.Filters
	
	return nil
}

// ShouldFire checks if the event trigger should fire
func (t *EventTrigger) ShouldFire(event interface{}) bool {
	if !t.IsActive() {
		return false
	}
	
	eventData, ok := event.(map[string]interface{})
	if !ok {
		return false
	}
	
	// Check event type
	if eventType, ok := eventData["type"].(string); ok {
		if eventType != t.EventType {
			return false
		}
	} else {
		return false
	}
	
	// Check event source if specified
	if t.EventSource != "" {
		if source, ok := eventData["source"].(string); ok {
			if source != t.EventSource {
				return false
			}
		} else {
			return false
		}
	}
	
	// Apply filters
	for key, expectedValue := range t.Filters {
		actualValue, exists := eventData[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}
	
	return true
}

// ManualTrigger represents a manual trigger
type ManualTrigger struct {
	BaseTrigger
	RequireConfirmation bool     `json:"requireConfirmation"`
	AllowedUsers        []string `json:"allowedUsers"`
}

// NewManualTrigger creates a new manual trigger
func NewManualTrigger(workflowID, name string) *ManualTrigger {
	return &ManualTrigger{
		BaseTrigger: BaseTrigger{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			Type:       TriggerTypeManual,
			Name:       name,
			Status:     TriggerStatusActive, // Manual triggers are always active
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Config:     make(map[string]interface{}),
		},
		RequireConfirmation: false,
		AllowedUsers:        []string{},
	}
}

// Validate validates the manual trigger
func (t *ManualTrigger) Validate() error {
	// Update config
	t.Config["requireConfirmation"] = t.RequireConfirmation
	t.Config["allowedUsers"] = t.AllowedUsers
	
	return nil
}

// ShouldFire checks if the manual trigger should fire
func (t *ManualTrigger) ShouldFire(event interface{}) bool {
	manualEvent, ok := event.(map[string]interface{})
	if !ok {
		return false
	}
	
	// Check if user is allowed
	if len(t.AllowedUsers) > 0 {
		userID, ok := manualEvent["userId"].(string)
		if !ok {
			return false
		}
		
		allowed := false
		for _, allowedUser := range t.AllowedUsers {
			if allowedUser == userID {
				allowed = true
				break
			}
		}
		
		if !allowed {
			return false
		}
	}
	
	// Check confirmation if required
	if t.RequireConfirmation {
		confirmed, ok := manualEvent["confirmed"].(bool)
		if !ok || !confirmed {
			return false
		}
	}
	
	return true
}

// EmailTrigger represents an email trigger
type EmailTrigger struct {
	BaseTrigger
	EmailAddress string   `json:"emailAddress"`
	Subject      string   `json:"subject"`
	FromFilter   []string `json:"fromFilter"`
	Keywords     []string `json:"keywords"`
}

// NewEmailTrigger creates a new email trigger
func NewEmailTrigger(workflowID, name, emailAddress string) *EmailTrigger {
	return &EmailTrigger{
		BaseTrigger: BaseTrigger{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			Type:       TriggerTypeEmail,
			Name:       name,
			Status:     TriggerStatusInactive,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Config:     make(map[string]interface{}),
		},
		EmailAddress: emailAddress,
		FromFilter:   []string{},
		Keywords:     []string{},
	}
}

// Validate validates the email trigger
func (t *EmailTrigger) Validate() error {
	if t.EmailAddress == "" {
		return errors.New("email address is required")
	}
	
	// Basic email validation
	if len(t.EmailAddress) < 3 || len(t.EmailAddress) > 254 {
		return errors.New("invalid email address")
	}
	
	// Update config
	t.Config["emailAddress"] = t.EmailAddress
	t.Config["subject"] = t.Subject
	t.Config["fromFilter"] = t.FromFilter
	t.Config["keywords"] = t.Keywords
	
	return nil
}

// ShouldFire checks if the email trigger should fire
func (t *EmailTrigger) ShouldFire(event interface{}) bool {
	if !t.IsActive() {
		return false
	}
	
	emailEvent, ok := event.(map[string]interface{})
	if !ok {
		return false
	}
	
	// Check email address
	if to, ok := emailEvent["to"].(string); ok {
		if to != t.EmailAddress {
			return false
		}
	} else {
		return false
	}
	
	// Check subject filter
	if t.Subject != "" {
		if subject, ok := emailEvent["subject"].(string); ok {
			// Simple substring match
			if !containsString(subject, t.Subject) {
				return false
			}
		}
	}
	
	// Check from filter
	if len(t.FromFilter) > 0 {
		from, ok := emailEvent["from"].(string)
		if !ok {
			return false
		}
		
		matched := false
		for _, allowedFrom := range t.FromFilter {
			if from == allowedFrom {
				matched = true
				break
			}
		}
		
		if !matched {
			return false
		}
	}
	
	// Check keywords in body
	if len(t.Keywords) > 0 {
		body, ok := emailEvent["body"].(string)
		if !ok {
			return false
		}
		
		for _, keyword := range t.Keywords {
			if !containsString(body, keyword) {
				return false
			}
		}
	}
	
	return true
}

// Helper function for string contains
func containsString(str, substr string) bool {
	return len(str) >= len(substr) && str[:len(substr)] == substr
}

// TriggerFactory creates trigger instances
type TriggerFactory struct{}

// NewTriggerFactory creates a new trigger factory
func NewTriggerFactory() *TriggerFactory {
	return &TriggerFactory{}
}

// CreateTrigger creates a trigger instance based on type
func (f *TriggerFactory) CreateTrigger(triggerType string, config map[string]interface{}) (Trigger, error) {
	workflowID, _ := config["workflowId"].(string)
	name, _ := config["name"].(string)
	
	switch triggerType {
	case TriggerTypeWebhook:
		path, _ := config["path"].(string)
		trigger := NewWebhookTrigger(workflowID, name, path)
		if method, ok := config["method"].(string); ok {
			trigger.Method = method
		}
		if secret, ok := config["secret"].(string); ok {
			trigger.Secret = secret
		}
		return trigger, nil
		
	case TriggerTypeSchedule:
		cronExpr, _ := config["cronExpression"].(string)
		trigger := NewScheduleTrigger(workflowID, name, cronExpr)
		if tz, ok := config["timezone"].(string); ok {
			trigger.Timezone = tz
		}
		return trigger, nil
		
	case TriggerTypeEvent:
		eventType, _ := config["eventType"].(string)
		trigger := NewEventTrigger(workflowID, name, eventType)
		if source, ok := config["eventSource"].(string); ok {
			trigger.EventSource = source
		}
		if filters, ok := config["filters"].(map[string]interface{}); ok {
			trigger.Filters = filters
		}
		return trigger, nil
		
	case TriggerTypeManual:
		trigger := NewManualTrigger(workflowID, name)
		if confirm, ok := config["requireConfirmation"].(bool); ok {
			trigger.RequireConfirmation = confirm
		}
		if users, ok := config["allowedUsers"].([]string); ok {
			trigger.AllowedUsers = users
		}
		return trigger, nil
		
	case TriggerTypeEmail:
		email, _ := config["emailAddress"].(string)
		trigger := NewEmailTrigger(workflowID, name, email)
		if subject, ok := config["subject"].(string); ok {
			trigger.Subject = subject
		}
		if from, ok := config["fromFilter"].([]string); ok {
			trigger.FromFilter = from
		}
		if keywords, ok := config["keywords"].([]string); ok {
			trigger.Keywords = keywords
		}
		return trigger, nil
		
	default:
		return nil, fmt.Errorf("unsupported trigger type: %s", triggerType)
	}
}
