package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// Store handles persistence of execution state and checkpoints
type Store struct {
	db       *sql.DB
	redis    *redis.Client
	eventBus events.EventBus
	logger   logger.Logger

	// Configuration
	checkpointTTL      time.Duration
	maxCheckpoints     int
	compressionEnabled bool

	// Checkpointing
	checkpointQueue chan *Checkpoint
	mu              sync.RWMutex

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// Checkpoint represents an execution checkpoint
type Checkpoint struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id"`
	State       ExecutionState         `json:"state"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     int                    `json:"version"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ExecutionState represents the complete state of an execution
type ExecutionState struct {
	ExecutionID    string                 `json:"execution_id"`
	WorkflowID     string                 `json:"workflow_id"`
	Status         string                 `json:"status"`
	Context        map[string]interface{} `json:"context"`
	NodeOutputs    map[string]interface{} `json:"node_outputs"`
	CompletedNodes []string               `json:"completed_nodes"`
	PendingNodes   []string               `json:"pending_nodes"`
	Variables      map[string]interface{} `json:"variables"`
	Errors         []ExecutionError       `json:"errors"`
	StartTime      time.Time              `json:"start_time"`
	LastCheckpoint time.Time              `json:"last_checkpoint"`
}

// ExecutionError represents an error during execution
type ExecutionError struct {
	NodeID    string    `json:"node_id"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	Retryable bool      `json:"retryable"`
}

// StoreConfig contains configuration for the store
type StoreConfig struct {
	CheckpointTTL      time.Duration
	MaxCheckpoints     int
	CompressionEnabled bool
	BatchSize          int
}

// NewStore creates a new persistence store
func NewStore(
	db *sql.DB,
	redis *redis.Client,
	eventBus events.EventBus,
	config StoreConfig,
	logger logger.Logger,
) *Store {
	if config.CheckpointTTL == 0 {
		config.CheckpointTTL = 7 * 24 * time.Hour // 7 days
	}
	if config.MaxCheckpoints == 0 {
		config.MaxCheckpoints = 100
	}

	return &Store{
		db:                 db,
		redis:              redis,
		eventBus:           eventBus,
		logger:             logger,
		checkpointTTL:      config.CheckpointTTL,
		maxCheckpoints:     config.MaxCheckpoints,
		compressionEnabled: config.CompressionEnabled,
		checkpointQueue:    make(chan *Checkpoint, 1000),
		stopCh:             make(chan struct{}),
	}
}

// Start starts the persistence store
func (s *Store) Start(ctx context.Context) error {
	s.logger.Info("Starting persistence store")

	// Start checkpoint processor
	s.wg.Add(1)
	go s.processCheckpoints(ctx)

	// Start cleanup task
	s.wg.Add(1)
	go s.cleanupOldCheckpoints(ctx)

	return nil
}

// Stop stops the persistence store
func (s *Store) Stop(ctx context.Context) error {
	s.logger.Info("Stopping persistence store")

	close(s.stopCh)
	close(s.checkpointQueue)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Persistence store stopped")
	case <-ctx.Done():
		s.logger.Warn("Persistence store stop timeout")
	}

	return nil
}

// SaveCheckpoint saves an execution checkpoint
func (s *Store) SaveCheckpoint(ctx context.Context, checkpoint *Checkpoint) error {
	if checkpoint.ID == "" {
		checkpoint.ID = uuid.New().String()
	}
	checkpoint.Timestamp = time.Now()

	// Add to queue for async processing
	select {
	case s.checkpointQueue <- checkpoint:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("checkpoint queue full")
	}
}

// SaveCheckpointSync saves a checkpoint synchronously
func (s *Store) SaveCheckpointSync(ctx context.Context, checkpoint *Checkpoint) error {
	// Save to database
	if err := s.saveToDatabase(ctx, checkpoint); err != nil {
		return fmt.Errorf("failed to save to database: %w", err)
	}

	// Save to Redis for fast access
	if err := s.saveToRedis(ctx, checkpoint); err != nil {
		s.logger.Error("Failed to save checkpoint to Redis", "error", err)
		// Don't fail if Redis save fails
	}

	// Publish checkpoint event
	event := events.NewEventBuilder("checkpoint.saved").
		WithAggregateID(checkpoint.ExecutionID).
		WithPayload("checkpointId", checkpoint.ID).
		WithPayload("nodeId", checkpoint.NodeID).
		Build()

	s.eventBus.Publish(ctx, event)

	return nil
}

// GetLatestCheckpoint gets the latest checkpoint for an execution
func (s *Store) GetLatestCheckpoint(ctx context.Context, executionID string) (*Checkpoint, error) {
	// Try Redis first
	checkpoint, err := s.getFromRedis(ctx, executionID)
	if err == nil {
		return checkpoint, nil
	}

	// Fallback to database
	return s.getFromDatabase(ctx, executionID)
}

// GetCheckpointByID gets a specific checkpoint
func (s *Store) GetCheckpointByID(ctx context.Context, checkpointID string) (*Checkpoint, error) {
	query := `
		SELECT id, execution_id, node_id, state, timestamp, version, metadata
		FROM execution_checkpoints
		WHERE id = $1
	`

	var checkpoint Checkpoint
	var stateJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, checkpointID).Scan(
		&checkpoint.ID,
		&checkpoint.ExecutionID,
		&checkpoint.NodeID,
		&stateJSON,
		&checkpoint.Timestamp,
		&checkpoint.Version,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(stateJSON, &checkpoint.State); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &checkpoint.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &checkpoint, nil
}

// ListCheckpoints lists all checkpoints for an execution
func (s *Store) ListCheckpoints(ctx context.Context, executionID string) ([]*Checkpoint, error) {
	query := `
		SELECT id, execution_id, node_id, state, timestamp, version, metadata
		FROM execution_checkpoints
		WHERE execution_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, query, executionID, s.maxCheckpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to list checkpoints: %w", err)
	}
	defer rows.Close()

	var checkpoints []*Checkpoint

	for rows.Next() {
		var checkpoint Checkpoint
		var stateJSON, metadataJSON []byte

		err := rows.Scan(
			&checkpoint.ID,
			&checkpoint.ExecutionID,
			&checkpoint.NodeID,
			&stateJSON,
			&checkpoint.Timestamp,
			&checkpoint.Version,
			&metadataJSON,
		)

		if err != nil {
			s.logger.Error("Failed to scan checkpoint", "error", err)
			continue
		}

		// Unmarshal JSON fields
		json.Unmarshal(stateJSON, &checkpoint.State)
		json.Unmarshal(metadataJSON, &checkpoint.Metadata)

		checkpoints = append(checkpoints, &checkpoint)
	}

	return checkpoints, nil
}

// DeleteCheckpoint deletes a checkpoint
func (s *Store) DeleteCheckpoint(ctx context.Context, checkpointID string) error {
	// Delete from database
	query := `DELETE FROM execution_checkpoints WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, checkpointID)
	if err != nil {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	// Delete from Redis
	s.deleteFromRedis(ctx, checkpointID)

	return nil
}

// SaveExecutionState saves the complete execution state
func (s *Store) SaveExecutionState(ctx context.Context, state *ExecutionState) error {
	// Create a checkpoint from the state
	checkpoint := &Checkpoint{
		ID:          uuid.New().String(),
		ExecutionID: state.ExecutionID,
		NodeID:      "", // Full execution state
		State:       *state,
		Timestamp:   time.Now(),
		Version:     1,
		Metadata: map[string]interface{}{
			"type": "full_state",
		},
	}

	return s.SaveCheckpointSync(ctx, checkpoint)
}

// GetExecutionState gets the complete execution state
func (s *Store) GetExecutionState(ctx context.Context, executionID string) (*ExecutionState, error) {
	checkpoint, err := s.GetLatestCheckpoint(ctx, executionID)
	if err != nil {
		return nil, err
	}

	return &checkpoint.State, nil
}

// processCheckpoints processes checkpoints from the queue
func (s *Store) processCheckpoints(ctx context.Context) {
	defer s.wg.Done()

	batch := make([]*Checkpoint, 0, 10)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case checkpoint, ok := <-s.checkpointQueue:
			if !ok {
				// Queue closed, save remaining batch
				if len(batch) > 0 {
					s.saveBatch(ctx, batch)
				}
				return
			}

			batch = append(batch, checkpoint)

			// Save batch if full
			if len(batch) >= 10 {
				s.saveBatch(ctx, batch)
				batch = make([]*Checkpoint, 0, 10)
			}

		case <-ticker.C:
			// Save batch periodically
			if len(batch) > 0 {
				s.saveBatch(ctx, batch)
				batch = make([]*Checkpoint, 0, 10)
			}

		case <-s.stopCh:
			// Save remaining batch
			if len(batch) > 0 {
				s.saveBatch(ctx, batch)
			}
			return
		}
	}
}

// saveBatch saves a batch of checkpoints
func (s *Store) saveBatch(ctx context.Context, checkpoints []*Checkpoint) {
	for _, checkpoint := range checkpoints {
		if err := s.SaveCheckpointSync(ctx, checkpoint); err != nil {
			s.logger.Error("Failed to save checkpoint",
				"checkpointId", checkpoint.ID,
				"executionId", checkpoint.ExecutionID,
				"error", err,
			)
		}
	}

	s.logger.Debug("Saved checkpoint batch", "count", len(checkpoints))
}

// saveToDatabase saves a checkpoint to the database
func (s *Store) saveToDatabase(ctx context.Context, checkpoint *Checkpoint) error {
	stateJSON, err := json.Marshal(checkpoint.State)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	metadataJSON, err := json.Marshal(checkpoint.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO execution_checkpoints 
		(id, execution_id, node_id, state, timestamp, version, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			state = EXCLUDED.state,
			timestamp = EXCLUDED.timestamp,
			version = EXCLUDED.version,
			metadata = EXCLUDED.metadata
	`

	_, err = s.db.ExecContext(ctx, query,
		checkpoint.ID,
		checkpoint.ExecutionID,
		checkpoint.NodeID,
		stateJSON,
		checkpoint.Timestamp,
		checkpoint.Version,
		metadataJSON,
	)

	return err
}

// saveToRedis saves a checkpoint to Redis
func (s *Store) saveToRedis(ctx context.Context, checkpoint *Checkpoint) error {
	key := fmt.Sprintf("checkpoint:%s:latest", checkpoint.ExecutionID)

	data, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	return s.redis.Set(ctx, key, data, s.checkpointTTL).Err()
}

// getFromRedis gets a checkpoint from Redis
func (s *Store) getFromRedis(ctx context.Context, executionID string) (*Checkpoint, error) {
	key := fmt.Sprintf("checkpoint:%s:latest", executionID)

	data, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal([]byte(data), &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// getFromDatabase gets the latest checkpoint from the database
func (s *Store) getFromDatabase(ctx context.Context, executionID string) (*Checkpoint, error) {
	query := `
		SELECT id, execution_id, node_id, state, timestamp, version, metadata
		FROM execution_checkpoints
		WHERE execution_id = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var checkpoint Checkpoint
	var stateJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, executionID).Scan(
		&checkpoint.ID,
		&checkpoint.ExecutionID,
		&checkpoint.NodeID,
		&stateJSON,
		&checkpoint.Timestamp,
		&checkpoint.Version,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no checkpoint found for execution: %s", executionID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(stateJSON, &checkpoint.State); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &checkpoint.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &checkpoint, nil
}

// deleteFromRedis deletes a checkpoint from Redis
func (s *Store) deleteFromRedis(ctx context.Context, checkpointID string) {
	// Find and delete the checkpoint
	// This is simplified - in production, would maintain proper index
	iter := s.redis.Scan(ctx, 0, "checkpoint:*", 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		data, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var checkpoint Checkpoint
		if err := json.Unmarshal([]byte(data), &checkpoint); err != nil {
			continue
		}

		if checkpoint.ID == checkpointID {
			s.redis.Del(ctx, key)
			break
		}
	}
}

// cleanupOldCheckpoints periodically cleans up old checkpoints
func (s *Store) cleanupOldCheckpoints(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.performCleanup(ctx)
		case <-s.stopCh:
			return
		}
	}
}

// performCleanup performs checkpoint cleanup
func (s *Store) performCleanup(ctx context.Context) {
	cutoffTime := time.Now().Add(-s.checkpointTTL)

	query := `
		DELETE FROM execution_checkpoints
		WHERE timestamp < $1
	`

	result, err := s.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		s.logger.Error("Failed to cleanup old checkpoints", "error", err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		s.logger.Info("Cleaned up old checkpoints", "count", rowsAffected)
	}
}
