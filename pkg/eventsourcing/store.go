package eventsourcing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EventStore defines the interface for event storage
type EventStore interface {
	// Save stores events in the event store
	Save(ctx context.Context, events []Event) error

	// Load retrieves all events for an aggregate
	Load(ctx context.Context, aggregateID string) ([]Event, error)

	// LoadFromVersion retrieves events starting from a specific version
	LoadFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]Event, error)

	// LoadSnapshot retrieves the latest snapshot for an aggregate
	LoadSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)

	// SaveSnapshot stores a snapshot
	SaveSnapshot(ctx context.Context, snapshot *Snapshot) error

	// GetAggregateVersion returns the current version of an aggregate
	GetAggregateVersion(ctx context.Context, aggregateID string) (int, error)
}

// Event represents a domain event
type Event struct {
	ID            string            `json:"id" gorm:"primaryKey"`
	AggregateID   string            `json:"aggregateId" gorm:"not null;index"`
	Type          string            `json:"type" gorm:"not null;index"`
	Version       int               `json:"version" gorm:"not null"`
	Payload       json.RawMessage   `json:"payload" gorm:"type:jsonb"`
	Metadata      map[string]string `json:"metadata" gorm:"serializer:json"`
	Timestamp     time.Time         `json:"timestamp" gorm:"not null;index"`
	UserID        string            `json:"userId" gorm:"index"`
	CorrelationID string            `json:"correlationId" gorm:"index"`
}

// Snapshot represents an aggregate snapshot
type Snapshot struct {
	ID          string          `json:"id" gorm:"primaryKey"`
	AggregateID string          `json:"aggregateId" gorm:"not null;uniqueIndex"`
	Version     int             `json:"version" gorm:"not null"`
	State       json.RawMessage `json:"state" gorm:"type:jsonb"`
	Timestamp   time.Time       `json:"timestamp" gorm:"not null"`
}

// GormEventStore implements EventStore using GORM
type GormEventStore struct {
	db                *gorm.DB
	snapshotFrequency int
}

// NewGormEventStore creates a new GORM-based event store
func NewGormEventStore(db *gorm.DB, snapshotFrequency int) *GormEventStore {
	return &GormEventStore{
		db:                db,
		snapshotFrequency: snapshotFrequency,
	}
}

// Save stores events in the event store
func (s *GormEventStore) Save(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Verify version continuity
		aggregateID := events[0].AggregateID
		currentVersion, err := s.getVersionInTx(tx, aggregateID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		expectedVersion := currentVersion
		for i, event := range events {
			expectedVersion++
			if event.Version != expectedVersion {
				return fmt.Errorf("version mismatch: expected %d, got %d for event %d",
					expectedVersion, event.Version, i)
			}

			// Set event ID if not provided
			if event.ID == "" {
				event.ID = uuid.New().String()
			}

			// Set timestamp if not provided
			if event.Timestamp.IsZero() {
				event.Timestamp = time.Now()
			}

			// Save event
			if err := tx.Create(&event).Error; err != nil {
				return fmt.Errorf("failed to save event: %w", err)
			}
		}

		// Check if we need to create a snapshot
		if s.snapshotFrequency > 0 && expectedVersion%s.snapshotFrequency == 0 {
			// This would typically trigger snapshot creation asynchronously
			// For now, we'll just mark it as needed
			_ = s.markSnapshotNeeded(tx, aggregateID, expectedVersion)
		}

		return nil
	})
}

// Load retrieves all events for an aggregate
func (s *GormEventStore) Load(ctx context.Context, aggregateID string) ([]Event, error) {
	var events []Event
	err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		Order("version ASC").
		Find(&events).Error

	return events, err
}

// LoadFromVersion retrieves events starting from a specific version
func (s *GormEventStore) LoadFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]Event, error) {
	var events []Event
	err := s.db.WithContext(ctx).
		Where("aggregate_id = ? AND version > ?", aggregateID, fromVersion).
		Order("version ASC").
		Find(&events).Error

	return events, err
}

// LoadSnapshot retrieves the latest snapshot for an aggregate
func (s *GormEventStore) LoadSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error) {
	var snapshot Snapshot
	err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		Order("version DESC").
		First(&snapshot).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &snapshot, err
}

// SaveSnapshot stores a snapshot
func (s *GormEventStore) SaveSnapshot(ctx context.Context, snapshot *Snapshot) error {
	if snapshot.ID == "" {
		snapshot.ID = uuid.New().String()
	}

	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete old snapshots (keep only the latest)
		if err := tx.Where("aggregate_id = ?", snapshot.AggregateID).
			Delete(&Snapshot{}).Error; err != nil {
			return err
		}

		// Save new snapshot
		return tx.Create(snapshot).Error
	})
}

// GetAggregateVersion returns the current version of an aggregate
func (s *GormEventStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int, error) {
	var maxVersion int
	err := s.db.WithContext(ctx).
		Model(&Event{}).
		Where("aggregate_id = ?", aggregateID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error

	return maxVersion, err
}

func (s *GormEventStore) getVersionInTx(tx *gorm.DB, aggregateID string) (int, error) {
	var maxVersion int
	err := tx.Model(&Event{}).
		Where("aggregate_id = ?", aggregateID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error

	return maxVersion, err
}

func (s *GormEventStore) markSnapshotNeeded(tx *gorm.DB, aggregateID string, version int) error {
	// This is a placeholder for marking that a snapshot is needed
	// In a real implementation, this might add to a queue or set a flag
	return nil
}

// EventBus defines the interface for publishing events
type EventBus interface {
	// Publish sends an event to all subscribers
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a handler for specific event types
	Subscribe(eventType string, handler EventHandler) error

	// Unsubscribe removes a handler
	Unsubscribe(eventType string, handler EventHandler) error
}

// EventHandler is a function that handles events
type EventHandler func(ctx context.Context, event Event) error

// EventStream represents a stream of events
type EventStream struct {
	AggregateID string
	Events      []Event
	Version     int
}

// EventQuery represents query criteria for events
type EventQuery struct {
	AggregateID   string
	AggregateType string
	EventTypes    []string
	FromVersion   int
	ToVersion     int
	FromTimestamp time.Time
	ToTimestamp   time.Time
	UserID        string
	Limit         int
	Offset        int
}

// QueryEvents queries events based on criteria
func (s *GormEventStore) QueryEvents(ctx context.Context, query EventQuery) ([]Event, error) {
	q := s.db.WithContext(ctx).Model(&Event{})

	if query.AggregateID != "" {
		q = q.Where("aggregate_id = ?", query.AggregateID)
	}

	if len(query.EventTypes) > 0 {
		q = q.Where("type IN ?", query.EventTypes)
	}

	if query.FromVersion > 0 {
		q = q.Where("version >= ?", query.FromVersion)
	}

	if query.ToVersion > 0 {
		q = q.Where("version <= ?", query.ToVersion)
	}

	if !query.FromTimestamp.IsZero() {
		q = q.Where("timestamp >= ?", query.FromTimestamp)
	}

	if !query.ToTimestamp.IsZero() {
		q = q.Where("timestamp <= ?", query.ToTimestamp)
	}

	if query.UserID != "" {
		q = q.Where("user_id = ?", query.UserID)
	}

	q = q.Order("timestamp ASC, version ASC")

	if query.Limit > 0 {
		q = q.Limit(query.Limit)
	}

	if query.Offset > 0 {
		q = q.Offset(query.Offset)
	}

	var events []Event
	err := q.Find(&events).Error

	return events, err
}

// GetEventStats returns statistics about events
func (s *GormEventStore) GetEventStats(ctx context.Context, aggregateID string) (*EventStats, error) {
	stats := &EventStats{
		AggregateID: aggregateID,
	}

	// Total events
	err := s.db.WithContext(ctx).
		Model(&Event{}).
		Where("aggregate_id = ?", aggregateID).
		Count(&stats.TotalEvents).Error
	if err != nil {
		return nil, err
	}

	// First and last event times
	var firstEvent, lastEvent Event

	if err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		Order("timestamp ASC").
		First(&firstEvent).Error; err == nil {
		stats.FirstEventTime = &firstEvent.Timestamp
	}

	if err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		Order("timestamp DESC").
		First(&lastEvent).Error; err == nil {
		stats.LastEventTime = &lastEvent.Timestamp
		stats.CurrentVersion = lastEvent.Version
	}

	// Event type counts
	type TypeCount struct {
		Type  string
		Count int64
	}

	var typeCounts []TypeCount
	s.db.WithContext(ctx).
		Model(&Event{}).
		Select("type, COUNT(*) as count").
		Where("aggregate_id = ?", aggregateID).
		Group("type").
		Scan(&typeCounts)

	stats.EventTypeCounts = make(map[string]int64)
	for _, tc := range typeCounts {
		stats.EventTypeCounts[tc.Type] = tc.Count
	}

	// Snapshot info
	if snapshot, err := s.LoadSnapshot(ctx, aggregateID); err == nil && snapshot != nil {
		stats.LastSnapshotVersion = &snapshot.Version
		stats.LastSnapshotTime = &snapshot.Timestamp
	}

	return stats, nil
}

// EventStats contains statistics about events for an aggregate
type EventStats struct {
	AggregateID         string
	TotalEvents         int64
	CurrentVersion      int
	EventTypeCounts     map[string]int64
	FirstEventTime      *time.Time
	LastEventTime       *time.Time
	LastSnapshotVersion *int
	LastSnapshotTime    *time.Time
}
