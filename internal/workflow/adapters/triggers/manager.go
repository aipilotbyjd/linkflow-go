package triggers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var (
	ErrTriggerNotFound      = errors.New("trigger not found")
	ErrInvalidTriggerType   = errors.New("invalid trigger type")
	ErrTriggerAlreadyActive = errors.New("trigger already active")
	ErrTriggerNotActive     = errors.New("trigger not active")
	ErrWorkflowNotActive    = errors.New("workflow not active")
	ErrDuplicateTrigger     = errors.New("duplicate trigger exists")
)

// TriggerManager manages workflow triggers
type TriggerManager struct {
	db            *database.DB
	redis         *redis.Client
	eventBus      events.EventBus
	logger        logger.Logger
	factory       *workflow.TriggerFactory
	cronScheduler *cron.Cron
	webhooks      map[string]*workflow.WebhookTrigger
	schedules     map[string]*cron.EntryID
	mu            sync.RWMutex
	shutdownCh    chan struct{}
}

// NewTriggerManager creates a new trigger manager
func NewTriggerManager(db *database.DB, redis *redis.Client, eventBus events.EventBus, logger logger.Logger) *TriggerManager {
	return &TriggerManager{
		db:            db,
		redis:         redis,
		eventBus:      eventBus,
		logger:        logger,
		factory:       workflow.NewTriggerFactory(),
		cronScheduler: cron.New(cron.WithLocation(time.UTC)),
		webhooks:      make(map[string]*workflow.WebhookTrigger),
		schedules:     make(map[string]*cron.EntryID),
		shutdownCh:    make(chan struct{}),
	}
}

// Start starts the trigger manager
func (tm *TriggerManager) Start(ctx context.Context) error {
	tm.logger.Info("Starting trigger manager")

	// Start cron scheduler
	tm.cronScheduler.Start()

	// Load active triggers
	if err := tm.loadActiveTriggers(ctx); err != nil {
		return fmt.Errorf("failed to load active triggers: %w", err)
	}

	// Start event listener
	go tm.eventListener(ctx)

	// Start webhook server (would be separate in production)
	go tm.webhookListener(ctx)

	tm.logger.Info("Trigger manager started")
	return nil
}

// Stop stops the trigger manager
func (tm *TriggerManager) Stop(ctx context.Context) error {
	tm.logger.Info("Stopping trigger manager")

	close(tm.shutdownCh)

	// Stop cron scheduler
	tm.cronScheduler.Stop()

	// Clear active triggers
	tm.mu.Lock()
	tm.webhooks = make(map[string]*workflow.WebhookTrigger)
	tm.schedules = make(map[string]*cron.EntryID)
	tm.mu.Unlock()

	tm.logger.Info("Trigger manager stopped")
	return nil
}

// CreateTrigger creates a new trigger for a workflow
func (tm *TriggerManager) CreateTrigger(ctx context.Context, workflowID string, config map[string]interface{}) (*workflow.WorkflowTrigger, error) {
	// Add workflow ID to config
	config["workflowId"] = workflowID

	// Get trigger type
	triggerType, ok := config["type"].(string)
	if !ok {
		return nil, ErrInvalidTriggerType
	}

	// Create trigger instance
	trigger, err := tm.factory.CreateTrigger(triggerType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create trigger: %w", err)
	}

	// Validate trigger
	if err := trigger.Validate(); err != nil {
		return nil, fmt.Errorf("trigger validation failed: %w", err)
	}

	// Check for duplicates
	if err := tm.checkDuplicateTrigger(ctx, workflowID, triggerType, config); err != nil {
		return nil, err
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(trigger.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create database record
	wt := &workflow.WorkflowTrigger{
		ID:          trigger.GetID(),
		WorkflowID:  workflowID,
		Type:        triggerType,
		Name:        config["name"].(string),
		Description: getStringFromConfig(config, "description"),
		Status:      workflow.TriggerStatusInactive,
		Config:      configJSON,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save to database
	if err := tm.db.WithContext(ctx).Create(wt).Error; err != nil {
		return nil, fmt.Errorf("failed to save trigger: %w", err)
	}

	// Publish trigger created event
	tm.publishEvent(ctx, "trigger.created", map[string]interface{}{
		"trigger_id":  wt.ID,
		"workflow_id": workflowID,
		"type":        triggerType,
	})

	tm.logger.Info("Trigger created",
		"trigger_id", wt.ID,
		"workflow_id", workflowID,
		"type", triggerType)

	return wt, nil
}

// GetTrigger retrieves a trigger by ID
func (tm *TriggerManager) GetTrigger(ctx context.Context, triggerID string) (*workflow.WorkflowTrigger, error) {
	var trigger workflow.WorkflowTrigger
	err := tm.db.WithContext(ctx).Where("id = ?", triggerID).First(&trigger).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrTriggerNotFound
	}
	return &trigger, err
}

// ListTriggers lists triggers for a workflow
func (tm *TriggerManager) ListTriggers(ctx context.Context, workflowID string) ([]*workflow.WorkflowTrigger, error) {
	var triggers []*workflow.WorkflowTrigger
	err := tm.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("created_at DESC").
		Find(&triggers).Error
	return triggers, err
}

// UpdateTrigger updates a trigger configuration
func (tm *TriggerManager) UpdateTrigger(ctx context.Context, triggerID string, updates map[string]interface{}) (*workflow.WorkflowTrigger, error) {
	// Get existing trigger
	trigger, err := tm.GetTrigger(ctx, triggerID)
	if err != nil {
		return nil, err
	}

	// Parse existing config
	var config map[string]interface{}
	if err := json.Unmarshal(trigger.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Merge updates
	for key, value := range updates {
		if key != "id" && key != "workflowId" && key != "type" {
			config[key] = value
		}
	}

	// Create and validate updated trigger
	updatedTrigger, err := tm.factory.CreateTrigger(trigger.Type, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create updated trigger: %w", err)
	}

	if err := updatedTrigger.Validate(); err != nil {
		return nil, fmt.Errorf("trigger validation failed: %w", err)
	}

	// Update config
	configJSON, err := json.Marshal(updatedTrigger.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Update database
	trigger.Config = configJSON
	trigger.UpdatedAt = time.Now()

	if err := tm.db.WithContext(ctx).Save(trigger).Error; err != nil {
		return nil, fmt.Errorf("failed to update trigger: %w", err)
	}

	// If trigger is active, reload it
	if trigger.Status == workflow.TriggerStatusActive {
		tm.deactivateTrigger(ctx, trigger)
		tm.activateTrigger(ctx, trigger)
	}

	// Publish event
	tm.publishEvent(ctx, "trigger.updated", map[string]interface{}{
		"trigger_id":  triggerID,
		"workflow_id": trigger.WorkflowID,
	})

	return trigger, nil
}

// DeleteTrigger deletes a trigger
func (tm *TriggerManager) DeleteTrigger(ctx context.Context, triggerID string) error {
	// Get trigger
	trigger, err := tm.GetTrigger(ctx, triggerID)
	if err != nil {
		return err
	}

	// Deactivate if active
	if trigger.Status == workflow.TriggerStatusActive {
		if err := tm.DeactivateTrigger(ctx, triggerID); err != nil {
			return fmt.Errorf("failed to deactivate trigger: %w", err)
		}
	}

	// Delete from database
	if err := tm.db.WithContext(ctx).Delete(&workflow.WorkflowTrigger{}, "id = ?", triggerID).Error; err != nil {
		return fmt.Errorf("failed to delete trigger: %w", err)
	}

	// Publish event
	tm.publishEvent(ctx, "trigger.deleted", map[string]interface{}{
		"trigger_id":  triggerID,
		"workflow_id": trigger.WorkflowID,
	})

	tm.logger.Info("Trigger deleted", "trigger_id", triggerID)
	return nil
}

// ActivateTrigger activates a trigger
func (tm *TriggerManager) ActivateTrigger(ctx context.Context, triggerID string) error {
	// Get trigger
	trigger, err := tm.GetTrigger(ctx, triggerID)
	if err != nil {
		return err
	}

	// Check if already active
	if trigger.Status == workflow.TriggerStatusActive {
		return ErrTriggerAlreadyActive
	}

	// Activate based on type
	if err := tm.activateTrigger(ctx, trigger); err != nil {
		return fmt.Errorf("failed to activate trigger: %w", err)
	}

	// Update status
	trigger.Status = workflow.TriggerStatusActive
	trigger.UpdatedAt = time.Now()

	if err := tm.db.WithContext(ctx).Save(trigger).Error; err != nil {
		return fmt.Errorf("failed to update trigger status: %w", err)
	}

	// Publish event
	tm.publishEvent(ctx, "trigger.activated", map[string]interface{}{
		"trigger_id":  triggerID,
		"workflow_id": trigger.WorkflowID,
	})

	tm.logger.Info("Trigger activated", "trigger_id", triggerID, "type", trigger.Type)
	return nil
}

// DeactivateTrigger deactivates a trigger
func (tm *TriggerManager) DeactivateTrigger(ctx context.Context, triggerID string) error {
	// Get trigger
	trigger, err := tm.GetTrigger(ctx, triggerID)
	if err != nil {
		return err
	}

	// Check if active
	if trigger.Status != workflow.TriggerStatusActive {
		return ErrTriggerNotActive
	}

	// Deactivate based on type
	if err := tm.deactivateTrigger(ctx, trigger); err != nil {
		return fmt.Errorf("failed to deactivate trigger: %w", err)
	}

	// Update status
	trigger.Status = workflow.TriggerStatusInactive
	trigger.UpdatedAt = time.Now()

	if err := tm.db.WithContext(ctx).Save(trigger).Error; err != nil {
		return fmt.Errorf("failed to update trigger status: %w", err)
	}

	// Publish event
	tm.publishEvent(ctx, "trigger.deactivated", map[string]interface{}{
		"trigger_id":  triggerID,
		"workflow_id": trigger.WorkflowID,
	})

	tm.logger.Info("Trigger deactivated", "trigger_id", triggerID, "type", trigger.Type)
	return nil
}

// TestTrigger tests a trigger
func (tm *TriggerManager) TestTrigger(ctx context.Context, triggerID string, testData map[string]interface{}) (map[string]interface{}, error) {
	// Get trigger
	trigger, err := tm.GetTrigger(ctx, triggerID)
	if err != nil {
		return nil, err
	}

	// Parse config
	var config map[string]interface{}
	if err := json.Unmarshal(trigger.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Create trigger instance
	triggerInstance, err := tm.factory.CreateTrigger(trigger.Type, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create trigger instance: %w", err)
	}

	// Test trigger
	shouldFire := triggerInstance.ShouldFire(testData)

	result := map[string]interface{}{
		"trigger_id":   triggerID,
		"would_fire":   shouldFire,
		"test_data":    testData,
		"trigger_type": trigger.Type,
		"config":       config,
	}

	// Log test
	tm.logger.Info("Trigger tested",
		"trigger_id", triggerID,
		"would_fire", shouldFire)

	return result, nil
}

// activateTrigger activates a specific trigger type
func (tm *TriggerManager) activateTrigger(ctx context.Context, trigger *workflow.WorkflowTrigger) error {
	var config map[string]interface{}
	if err := json.Unmarshal(trigger.Config, &config); err != nil {
		return err
	}

	switch trigger.Type {
	case workflow.TriggerTypeWebhook:
		return tm.activateWebhookTrigger(trigger, config)
	case workflow.TriggerTypeSchedule:
		return tm.activateScheduleTrigger(trigger, config)
	case workflow.TriggerTypeEvent:
		return tm.activateEventTrigger(trigger, config)
	case workflow.TriggerTypeManual:
		// Manual triggers don't need activation
		return nil
	case workflow.TriggerTypeEmail:
		return tm.activateEmailTrigger(trigger, config)
	default:
		return ErrInvalidTriggerType
	}
}

// deactivateTrigger deactivates a specific trigger type
func (tm *TriggerManager) deactivateTrigger(ctx context.Context, trigger *workflow.WorkflowTrigger) error {
	switch trigger.Type {
	case workflow.TriggerTypeWebhook:
		return tm.deactivateWebhookTrigger(trigger.ID)
	case workflow.TriggerTypeSchedule:
		return tm.deactivateScheduleTrigger(trigger.ID)
	case workflow.TriggerTypeEvent:
		return tm.deactivateEventTrigger(trigger.ID)
	case workflow.TriggerTypeManual:
		// Manual triggers don't need deactivation
		return nil
	case workflow.TriggerTypeEmail:
		return tm.deactivateEmailTrigger(trigger.ID)
	default:
		return ErrInvalidTriggerType
	}
}

// activateWebhookTrigger activates a webhook trigger
func (tm *TriggerManager) activateWebhookTrigger(trigger *workflow.WorkflowTrigger, config map[string]interface{}) error {
	webhook := workflow.NewWebhookTrigger(trigger.WorkflowID, trigger.Name, config["path"].(string))
	webhook.ID = trigger.ID

	if method, ok := config["method"].(string); ok {
		webhook.Method = method
	}
	if secret, ok := config["secret"].(string); ok {
		webhook.Secret = secret
	}

	tm.mu.Lock()
	tm.webhooks[trigger.ID] = webhook
	tm.mu.Unlock()

	// Register webhook endpoint (in production, this would be done via HTTP router)
	return nil
}

// deactivateWebhookTrigger deactivates a webhook trigger
func (tm *TriggerManager) deactivateWebhookTrigger(triggerID string) error {
	tm.mu.Lock()
	delete(tm.webhooks, triggerID)
	tm.mu.Unlock()
	return nil
}

// activateScheduleTrigger activates a schedule trigger
func (tm *TriggerManager) activateScheduleTrigger(trigger *workflow.WorkflowTrigger, config map[string]interface{}) error {
	cronExpr := config["cronExpression"].(string)

	// Add cron job
	entryID, err := tm.cronScheduler.AddFunc(cronExpr, func() {
		tm.fireScheduleTrigger(trigger.ID, trigger.WorkflowID)
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	tm.mu.Lock()
	tm.schedules[trigger.ID] = &entryID
	tm.mu.Unlock()

	return nil
}

// deactivateScheduleTrigger deactivates a schedule trigger
func (tm *TriggerManager) deactivateScheduleTrigger(triggerID string) error {
	tm.mu.Lock()
	if entryID, ok := tm.schedules[triggerID]; ok {
		tm.cronScheduler.Remove(*entryID)
		delete(tm.schedules, triggerID)
	}
	tm.mu.Unlock()
	return nil
}

// activateEventTrigger activates an event trigger
func (tm *TriggerManager) activateEventTrigger(trigger *workflow.WorkflowTrigger, config map[string]interface{}) error {
	// Subscribe to event bus
	eventType := config["eventType"].(string)

	// Store subscription in Redis
	key := fmt.Sprintf("trigger:event:%s:%s", eventType, trigger.ID)
	data, _ := json.Marshal(map[string]interface{}{
		"trigger_id":  trigger.ID,
		"workflow_id": trigger.WorkflowID,
		"config":      config,
	})

	return tm.redis.Set(context.Background(), key, string(data), 0).Err()
}

// deactivateEventTrigger deactivates an event trigger
func (tm *TriggerManager) deactivateEventTrigger(triggerID string) error {
	// Remove subscription from Redis
	pattern := fmt.Sprintf("trigger:event:*:%s", triggerID)
	keys := tm.redis.Keys(context.Background(), pattern).Val()

	for _, key := range keys {
		tm.redis.Del(context.Background(), key)
	}

	return nil
}

// activateEmailTrigger activates an email trigger
func (tm *TriggerManager) activateEmailTrigger(trigger *workflow.WorkflowTrigger, config map[string]interface{}) error {
	// Register email webhook/polling (implementation would depend on email service)
	key := fmt.Sprintf("trigger:email:%s", trigger.ID)
	data, _ := json.Marshal(map[string]interface{}{
		"trigger_id":  trigger.ID,
		"workflow_id": trigger.WorkflowID,
		"config":      config,
	})

	return tm.redis.Set(context.Background(), key, string(data), 0).Err()
}

// deactivateEmailTrigger deactivates an email trigger
func (tm *TriggerManager) deactivateEmailTrigger(triggerID string) error {
	key := fmt.Sprintf("trigger:email:%s", triggerID)
	return tm.redis.Del(context.Background(), key).Err()
}

// fireScheduleTrigger fires a schedule trigger
func (tm *TriggerManager) fireScheduleTrigger(triggerID, workflowID string) {
	ctx := context.Background()

	// Update last fired time
	tm.db.Model(&workflow.WorkflowTrigger{}).
		Where("id = ?", triggerID).
		Updates(map[string]interface{}{
			"last_fired": time.Now(),
			"fire_count": gorm.Expr("fire_count + 1"),
		})

	// Publish execution event
	tm.publishEvent(ctx, "trigger.fired", map[string]interface{}{
		"trigger_id":  triggerID,
		"workflow_id": workflowID,
		"type":        workflow.TriggerTypeSchedule,
		"data":        map[string]interface{}{"scheduled_time": time.Now()},
	})

	tm.logger.Info("Schedule trigger fired", "trigger_id", triggerID, "workflow_id", workflowID)
}

// loadActiveTriggers loads all active triggers on startup
func (tm *TriggerManager) loadActiveTriggers(ctx context.Context) error {
	var triggers []*workflow.WorkflowTrigger
	err := tm.db.WithContext(ctx).
		Where("status = ?", workflow.TriggerStatusActive).
		Find(&triggers).Error

	if err != nil {
		return err
	}

	for _, trigger := range triggers {
		if err := tm.activateTrigger(ctx, trigger); err != nil {
			tm.logger.Error("Failed to load active trigger",
				"trigger_id", trigger.ID,
				"error", err)
			// Continue loading other triggers
		}
	}

	tm.logger.Info("Loaded active triggers", "count", len(triggers))
	return nil
}

// eventListener listens for events
func (tm *TriggerManager) eventListener(ctx context.Context) {
	// Subscribe to workflow events
	// This would integrate with the event bus
}

// webhookListener listens for webhook requests
func (tm *TriggerManager) webhookListener(ctx context.Context) {
	// This would be implemented as part of the HTTP server
}

// checkDuplicateTrigger checks if a duplicate trigger exists
func (tm *TriggerManager) checkDuplicateTrigger(ctx context.Context, workflowID, triggerType string, config map[string]interface{}) error {
	// Check for specific duplicate conditions based on type
	switch triggerType {
	case workflow.TriggerTypeWebhook:
		path, _ := config["path"].(string)
		method, _ := config["method"].(string)

		var count int64
		tm.db.Model(&workflow.WorkflowTrigger{}).
			Where("workflow_id = ? AND type = ?", workflowID, triggerType).
			Where("config->>'path' = ? AND config->>'method' = ?", path, method).
			Count(&count)

		if count > 0 {
			return ErrDuplicateTrigger
		}
	}

	return nil
}

// publishEvent publishes an event to the event bus
func (tm *TriggerManager) publishEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	event := events.Event{
		Type:    eventType,
		Payload: data,
	}

	if err := tm.eventBus.Publish(ctx, event); err != nil {
		tm.logger.Warn("Failed to publish event",
			"type", eventType,
			"error", err)
	}
}

// getStringFromConfig safely gets a string from config
func getStringFromConfig(config map[string]interface{}, key string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return ""
}
