package eventsourcing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AggregateRoot is the base for all aggregates
type AggregateRoot struct {
	ID            string
	Version       int
	Changes       []Event
	EventHandlers map[string]EventHandler
	correlationID string
	userID        string
}

// NewAggregateRoot creates a new aggregate root
func NewAggregateRoot(id string) *AggregateRoot {
	if id == "" {
		id = uuid.New().String()
	}

	return &AggregateRoot{
		ID:            id,
		Version:       0,
		Changes:       []Event{},
		EventHandlers: make(map[string]EventHandler),
	}
}

// ApplyChange applies an event and records it as a change
func (a *AggregateRoot) ApplyChange(event Event) {
	event.AggregateID = a.ID
	event.Version = a.Version + 1
	event.Timestamp = time.Now()
	event.UserID = a.userID
	event.CorrelationID = a.correlationID

	a.applyEvent(event, true)
}

// applyEvent applies an event to the aggregate
func (a *AggregateRoot) applyEvent(event Event, isNew bool) {
	// Apply the event using registered handler
	if handler, ok := a.EventHandlers[event.Type]; ok {
		_ = handler(context.Background(), event)
	} else {
		// Try to use reflection to call a method
		a.applyEventViaReflection(event)
	}

	a.Version = event.Version

	if isNew {
		a.Changes = append(a.Changes, event)
	}
}

// applyEventViaReflection attempts to call a method based on event type
func (a *AggregateRoot) applyEventViaReflection(event Event) {
	methodName := "Apply" + toPascalCase(event.Type)
	method := reflect.ValueOf(a).MethodByName(methodName)

	if method.IsValid() {
		method.Call([]reflect.Value{reflect.ValueOf(event)})
	}
}

// LoadFromEvents rebuilds the aggregate from events
func (a *AggregateRoot) LoadFromEvents(events []Event) {
	for _, event := range events {
		a.applyEvent(event, false)
	}
}

// GetUncommittedChanges returns events that haven't been saved
func (a *AggregateRoot) GetUncommittedChanges() []Event {
	return a.Changes
}

// MarkChangesAsCommitted clears the list of uncommitted changes
func (a *AggregateRoot) MarkChangesAsCommitted() {
	a.Changes = []Event{}
}

// SetUserID sets the user ID for events
func (a *AggregateRoot) SetUserID(userID string) {
	a.userID = userID
}

// SetCorrelationID sets the correlation ID for events
func (a *AggregateRoot) SetCorrelationID(correlationID string) {
	a.correlationID = correlationID
}

// GetID returns the aggregate ID
func (a *AggregateRoot) GetID() string {
	return a.ID
}

// GetVersion returns the current version
func (a *AggregateRoot) GetVersion() int {
	return a.Version
}

// Repository provides common repository operations for aggregates
type Repository struct {
	eventStore       EventStore
	aggregateFactory AggregateFactory
}

// AggregateFactory creates new aggregate instances
type AggregateFactory func() Aggregate

// Aggregate interface that all aggregates must implement
type Aggregate interface {
	GetID() string
	GetVersion() int
	LoadFromEvents(events []Event)
	GetUncommittedChanges() []Event
	MarkChangesAsCommitted()
}

// NewRepository creates a new repository
func NewRepository(eventStore EventStore, factory AggregateFactory) *Repository {
	return &Repository{
		eventStore:       eventStore,
		aggregateFactory: factory,
	}
}

// Load loads an aggregate from the event store
func (r *Repository) Load(ctx context.Context, aggregateID string) (Aggregate, error) {
	// Try to load from snapshot first
	snapshot, err := r.eventStore.LoadSnapshot(ctx, aggregateID)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}

	aggregate := r.aggregateFactory()

	if snapshot != nil {
		// Load from snapshot
		if err := r.loadFromSnapshot(aggregate, snapshot); err != nil {
			return nil, fmt.Errorf("failed to load from snapshot: %w", err)
		}

		// Load events after snapshot
		events, err := r.eventStore.LoadFromVersion(ctx, aggregateID, snapshot.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to load events: %w", err)
		}

		aggregate.LoadFromEvents(events)
	} else {
		// Load all events
		events, err := r.eventStore.Load(ctx, aggregateID)
		if err != nil {
			return nil, fmt.Errorf("failed to load events: %w", err)
		}

		if len(events) == 0 {
			return nil, errors.New("aggregate not found")
		}

		aggregate.LoadFromEvents(events)
	}

	return aggregate, nil
}

// Save saves an aggregate to the event store
func (r *Repository) Save(ctx context.Context, aggregate Aggregate) error {
	changes := aggregate.GetUncommittedChanges()
	if len(changes) == 0 {
		return nil // No changes to save
	}

	if err := r.eventStore.Save(ctx, changes); err != nil {
		return fmt.Errorf("failed to save events: %w", err)
	}

	aggregate.MarkChangesAsCommitted()

	// Check if we should create a snapshot
	// This is typically done asynchronously in production
	if aggregate.GetVersion()%10 == 0 {
		_ = r.createSnapshot(ctx, aggregate)
	}

	return nil
}

// loadFromSnapshot loads aggregate state from a snapshot
func (r *Repository) loadFromSnapshot(aggregate Aggregate, snapshot *Snapshot) error {
	// This would typically deserialize the snapshot state into the aggregate
	// Implementation depends on the specific aggregate type
	return json.Unmarshal(snapshot.State, aggregate)
}

// createSnapshot creates a snapshot of the aggregate
func (r *Repository) createSnapshot(ctx context.Context, aggregate Aggregate) error {
	state, err := json.Marshal(aggregate)
	if err != nil {
		return err
	}

	snapshot := &Snapshot{
		AggregateID: aggregate.GetID(),
		Version:     aggregate.GetVersion(),
		State:       state,
		Timestamp:   time.Now(),
	}

	return r.eventStore.SaveSnapshot(ctx, snapshot)
}

// Saga represents a long-running process that coordinates between aggregates
type Saga struct {
	ID             string
	State          string
	StartedAt      time.Time
	CompletedAt    *time.Time
	Steps          []SagaStep
	CurrentStep    int
	CompensateFrom int
}

// SagaStep represents a step in a saga
type SagaStep struct {
	Name        string
	Execute     func(ctx context.Context) error
	Compensate  func(ctx context.Context) error
	Completed   bool
	CompletedAt *time.Time
	Error       error
}

// SagaCoordinator manages saga execution
type SagaCoordinator struct {
	eventStore EventStore
}

// NewSagaCoordinator creates a new saga coordinator
func NewSagaCoordinator(eventStore EventStore) *SagaCoordinator {
	return &SagaCoordinator{
		eventStore: eventStore,
	}
}

// Execute runs a saga
func (c *SagaCoordinator) Execute(ctx context.Context, saga *Saga) error {
	saga.StartedAt = time.Now()
	saga.State = "running"

	for i, step := range saga.Steps {
		saga.CurrentStep = i

		if err := step.Execute(ctx); err != nil {
			step.Error = err
			saga.State = "compensating"
			saga.CompensateFrom = i

			// Start compensation
			return c.compensate(ctx, saga)
		}

		step.Completed = true
		now := time.Now()
		step.CompletedAt = &now
		saga.Steps[i] = step

		// Record saga progress event
		c.recordSagaEvent(saga, fmt.Sprintf("completed_step_%s", step.Name))
	}

	saga.State = "completed"
	now := time.Now()
	saga.CompletedAt = &now

	return nil
}

// compensate runs compensation for failed saga
func (c *SagaCoordinator) compensate(ctx context.Context, saga *Saga) error {
	for i := saga.CompensateFrom; i >= 0; i-- {
		step := saga.Steps[i]

		if step.Compensate != nil {
			if err := step.Compensate(ctx); err != nil {
				// Log compensation failure but continue
				c.recordSagaEvent(saga, fmt.Sprintf("compensation_failed_%s", step.Name))
			} else {
				c.recordSagaEvent(saga, fmt.Sprintf("compensated_%s", step.Name))
			}
		}
	}

	saga.State = "compensated"
	now := time.Now()
	saga.CompletedAt = &now

	return fmt.Errorf("saga failed at step %d", saga.CompensateFrom)
}

// recordSagaEvent records a saga event
func (c *SagaCoordinator) recordSagaEvent(saga *Saga, eventType string) {
	event := Event{
		ID:          uuid.New().String(),
		AggregateID: saga.ID,
		Type:        eventType,
		Version:     1,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"saga_state":   saga.State,
			"current_step": fmt.Sprintf("%d", saga.CurrentStep),
		},
	}

	// Best effort - ignore errors
	_ = c.eventStore.Save(context.Background(), []Event{event})
}

// Helper function to convert snake_case to PascalCase
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// EventProjector handles event projection
type EventProjector interface {
	// Project processes an event and updates read models
	Project(ctx context.Context, event Event) error

	// Rebuild rebuilds all projections from events
	Rebuild(ctx context.Context) error

	// GetLastProcessedVersion returns the last processed event version
	GetLastProcessedVersion(ctx context.Context) (int, error)
}
