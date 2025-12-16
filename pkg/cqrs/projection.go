package cqrs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/eventsourcing"
	"gorm.io/gorm"
)

// Projection defines the interface for read model projections
type Projection interface {
	// Handle processes an event and updates the read model
	Handle(ctx context.Context, event eventsourcing.Event) error

	// Reset clears the projection and rebuilds from scratch
	Reset(ctx context.Context) error

	// GetName returns the projection name
	GetName() string
}

// ProjectionManager manages multiple projections
type ProjectionManager struct {
	projections  map[string]Projection
	eventStore   eventsourcing.EventStore
	db           *gorm.DB
	mu           sync.RWMutex
	lastPosition int64
	running      bool
	stopChan     chan struct{}
}

// NewProjectionManager creates a new projection manager
func NewProjectionManager(eventStore eventsourcing.EventStore, db *gorm.DB) *ProjectionManager {
	return &ProjectionManager{
		projections: make(map[string]Projection),
		eventStore:  eventStore,
		db:          db,
		stopChan:    make(chan struct{}),
	}
}

// Register registers a projection
func (pm *ProjectionManager) Register(projection Projection) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.projections[projection.GetName()] = projection
}

// Start begins processing events
func (pm *ProjectionManager) Start(ctx context.Context) error {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return errors.New("projection manager already running")
	}
	pm.running = true
	pm.mu.Unlock()

	// Load last processed position
	if err := pm.loadPosition(); err != nil {
		return err
	}

	// Start processing in background
	go pm.processEvents(ctx)

	return nil
}

// Stop stops processing events
func (pm *ProjectionManager) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		close(pm.stopChan)
		pm.running = false
	}
}

// processEvents continuously processes events
func (pm *ProjectionManager) processEvents(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopChan:
			return
		case <-ticker.C:
			// Query new events
			events, err := pm.queryNewEvents(ctx)
			if err != nil {
				// Log error and continue
				continue
			}

			// Process events
			for _, event := range events {
				if err := pm.processEvent(ctx, event); err != nil {
					// Log error but continue processing
					continue
				}
			}
		}
	}
}

// processEvent processes a single event through all projections
func (pm *ProjectionManager) processEvent(ctx context.Context, event eventsourcing.Event) error {
	pm.mu.RLock()
	projections := make([]Projection, 0, len(pm.projections))
	for _, p := range pm.projections {
		projections = append(projections, p)
	}
	pm.mu.RUnlock()

	// Process event through each projection
	for _, projection := range projections {
		if err := projection.Handle(ctx, event); err != nil {
			// Log error but continue with other projections
			_ = err
		}
	}

	// Update position
	return pm.updatePosition(event)
}

// queryNewEvents queries for new events since last position
func (pm *ProjectionManager) queryNewEvents(ctx context.Context) ([]eventsourcing.Event, error) {
	// This would query events from the event store
	// For now, returning empty slice
	return []eventsourcing.Event{}, nil
}

// loadPosition loads the last processed position
func (pm *ProjectionManager) loadPosition() error {
	var position ProjectionPosition
	err := pm.db.Where("projection_name = ?", "main").First(&position).Error
	if err == nil {
		pm.lastPosition = position.Position
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

// updatePosition updates the last processed position
func (pm *ProjectionManager) updatePosition(event eventsourcing.Event) error {
	position := &ProjectionPosition{
		ProjectionName: "main",
		Position:       pm.lastPosition + 1,
		UpdatedAt:      time.Now(),
	}

	pm.lastPosition = position.Position

	return pm.db.Save(position).Error
}

// Rebuild rebuilds all projections from scratch
func (pm *ProjectionManager) Rebuild(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Reset all projections
	for _, projection := range pm.projections {
		if err := projection.Reset(ctx); err != nil {
			return fmt.Errorf("failed to reset projection %s: %w", projection.GetName(), err)
		}
	}

	// Reset position
	pm.lastPosition = 0

	// Query all events and replay
	// This would typically be done in batches for large event stores
	events := []eventsourcing.Event{} // Would load from event store

	for _, event := range events {
		for _, projection := range pm.projections {
			if err := projection.Handle(ctx, event); err != nil {
				return fmt.Errorf("failed to handle event in projection %s: %w",
					projection.GetName(), err)
			}
		}
	}

	return nil
}

// ProjectionPosition tracks the last processed event position
type ProjectionPosition struct {
	ProjectionName string    `gorm:"primaryKey"`
	Position       int64     `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

// WorkflowListProjection maintains a denormalized list of workflows
type WorkflowListProjection struct {
	db   *gorm.DB
	name string
}

// NewWorkflowListProjection creates a new workflow list projection
func NewWorkflowListProjection(db *gorm.DB) *WorkflowListProjection {
	return &WorkflowListProjection{
		db:   db,
		name: "workflow_list",
	}
}

// Handle processes an event
func (p *WorkflowListProjection) Handle(ctx context.Context, event eventsourcing.Event) error {
	switch event.Type {
	case "WorkflowCreated":
		return p.handleWorkflowCreated(ctx, event)
	case "WorkflowUpdated":
		return p.handleWorkflowUpdated(ctx, event)
	case "WorkflowDeleted":
		return p.handleWorkflowDeleted(ctx, event)
	case "WorkflowActivated":
		return p.handleWorkflowActivated(ctx, event)
	case "WorkflowDeactivated":
		return p.handleWorkflowDeactivated(ctx, event)
	}
	return nil
}

// Reset clears and rebuilds the projection
func (p *WorkflowListProjection) Reset(ctx context.Context) error {
	// Clear existing data
	if err := p.db.WithContext(ctx).
		Exec("TRUNCATE TABLE workflow_list_view").Error; err != nil {
		return err
	}
	return nil
}

// GetName returns the projection name
func (p *WorkflowListProjection) GetName() string {
	return p.name
}

func (p *WorkflowListProjection) handleWorkflowCreated(ctx context.Context, event eventsourcing.Event) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	view := &WorkflowListView{
		ID:          event.AggregateID,
		Name:        payload["name"].(string),
		Description: getStringValue(payload, "description"),
		UserID:      event.UserID,
		Status:      "inactive",
		IsActive:    false,
		CreatedAt:   event.Timestamp,
		UpdatedAt:   event.Timestamp,
	}

	return p.db.WithContext(ctx).Create(view).Error
}

func (p *WorkflowListProjection) handleWorkflowUpdated(ctx context.Context, event eventsourcing.Event) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	updates := map[string]interface{}{
		"updated_at": event.Timestamp,
	}

	if name, ok := payload["name"].(string); ok {
		updates["name"] = name
	}

	if description, ok := payload["description"].(string); ok {
		updates["description"] = description
	}

	return p.db.WithContext(ctx).
		Model(&WorkflowListView{}).
		Where("id = ?", event.AggregateID).
		Updates(updates).Error
}

func (p *WorkflowListProjection) handleWorkflowDeleted(ctx context.Context, event eventsourcing.Event) error {
	return p.db.WithContext(ctx).
		Where("id = ?", event.AggregateID).
		Delete(&WorkflowListView{}).Error
}

func (p *WorkflowListProjection) handleWorkflowActivated(ctx context.Context, event eventsourcing.Event) error {
	return p.db.WithContext(ctx).
		Model(&WorkflowListView{}).
		Where("id = ?", event.AggregateID).
		Updates(map[string]interface{}{
			"status":     "active",
			"is_active":  true,
			"updated_at": event.Timestamp,
		}).Error
}

func (p *WorkflowListProjection) handleWorkflowDeactivated(ctx context.Context, event eventsourcing.Event) error {
	return p.db.WithContext(ctx).
		Model(&WorkflowListView{}).
		Where("id = ?", event.AggregateID).
		Updates(map[string]interface{}{
			"status":     "inactive",
			"is_active":  false,
			"updated_at": event.Timestamp,
		}).Error
}

// WorkflowListView is the read model for workflow list
type WorkflowListView struct {
	ID                   string `gorm:"primaryKey"`
	Name                 string `gorm:"not null;index"`
	Description          string
	UserID               string `gorm:"not null;index"`
	TeamID               string `gorm:"index"`
	Status               string `gorm:"index"`
	IsActive             bool   `gorm:"index"`
	ExecutionCount       int    `gorm:"default:0"`
	LastExecutedAt       *time.Time
	SuccessRate          float64   `gorm:"default:0"`
	AverageExecutionTime int64     `gorm:"default:0"`
	Tags                 string    // JSON array stored as string
	CreatedAt            time.Time `gorm:"index"`
	UpdatedAt            time.Time `gorm:"index"`
}

// ExecutionStatsProjection maintains execution statistics
type ExecutionStatsProjection struct {
	db   *gorm.DB
	name string
}

// NewExecutionStatsProjection creates a new execution stats projection
func NewExecutionStatsProjection(db *gorm.DB) *ExecutionStatsProjection {
	return &ExecutionStatsProjection{
		db:   db,
		name: "execution_stats",
	}
}

// Handle processes an event
func (p *ExecutionStatsProjection) Handle(ctx context.Context, event eventsourcing.Event) error {
	switch event.Type {
	case "ExecutionStarted":
		return p.handleExecutionStarted(ctx, event)
	case "ExecutionCompleted":
		return p.handleExecutionCompleted(ctx, event)
	case "ExecutionFailed":
		return p.handleExecutionFailed(ctx, event)
	}
	return nil
}

// Reset clears and rebuilds the projection
func (p *ExecutionStatsProjection) Reset(ctx context.Context) error {
	return p.db.WithContext(ctx).
		Exec("TRUNCATE TABLE execution_stats_view").Error
}

// GetName returns the projection name
func (p *ExecutionStatsProjection) GetName() string {
	return p.name
}

func (p *ExecutionStatsProjection) handleExecutionStarted(ctx context.Context, event eventsourcing.Event) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	workflowID := payload["workflow_id"].(string)

	// Update workflow list view
	return p.db.WithContext(ctx).
		Model(&WorkflowListView{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"execution_count":  gorm.Expr("execution_count + ?", 1),
			"last_executed_at": event.Timestamp,
		}).Error
}

func (p *ExecutionStatsProjection) handleExecutionCompleted(ctx context.Context, event eventsourcing.Event) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	workflowID := payload["workflow_id"].(string)
	executionTime := int64(payload["execution_time"].(float64))

	// Get current stats
	var view WorkflowListView
	if err := p.db.WithContext(ctx).
		Where("id = ?", workflowID).
		First(&view).Error; err != nil {
		return err
	}

	// Calculate new average execution time
	newAvg := (view.AverageExecutionTime*int64(view.ExecutionCount) + executionTime) /
		int64(view.ExecutionCount+1)

	// Update stats
	return p.db.WithContext(ctx).
		Model(&WorkflowListView{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"average_execution_time": newAvg,
		}).Error
}

func (p *ExecutionStatsProjection) handleExecutionFailed(ctx context.Context, event eventsourcing.Event) error {
	// Update failure statistics
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	workflowID := payload["workflow_id"].(string)

	// This would update success rate calculation
	// Implementation depends on how you track success/failure counts
	_ = workflowID

	return nil
}

// Helper function to safely get string values from map
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// Query handlers for read models

// WorkflowListQuery queries the workflow list view
type WorkflowListQuery struct {
	db *gorm.DB
}

// NewWorkflowListQuery creates a new workflow list query handler
func NewWorkflowListQuery(db *gorm.DB) *WorkflowListQuery {
	return &WorkflowListQuery{db: db}
}

// List returns workflows from the read model
func (q *WorkflowListQuery) List(ctx context.Context, userID string, limit, offset int) ([]*WorkflowListView, error) {
	var workflows []*WorkflowListView

	query := q.db.WithContext(ctx)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	err := query.
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&workflows).Error

	return workflows, err
}

// GetByID returns a workflow by ID from the read model
func (q *WorkflowListQuery) GetByID(ctx context.Context, id string) (*WorkflowListView, error) {
	var workflow WorkflowListView
	err := q.db.WithContext(ctx).
		Where("id = ?", id).
		First(&workflow).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &workflow, err
}

// Search searches workflows in the read model
func (q *WorkflowListQuery) Search(ctx context.Context, query string) ([]*WorkflowListView, error) {
	var workflows []*WorkflowListView

	searchTerm := "%" + query + "%"
	err := q.db.WithContext(ctx).
		Where("name ILIKE ? OR description ILIKE ?", searchTerm, searchTerm).
		Order("updated_at DESC").
		Limit(50).
		Find(&workflows).Error

	return workflows, err
}
