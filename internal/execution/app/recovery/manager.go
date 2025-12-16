package recovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/internal/execution/app/orchestrator"
	"github.com/linkflow-go/internal/execution/app/persistence"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// Manager handles execution recovery from failures
type Manager struct {
	store        *persistence.Store
	orchestrator *orchestrator.Orchestrator
	eventBus     events.EventBus
	logger       logger.Logger

	// Recovery state
	recoveringExecutions map[string]*RecoveryTask
	mu                   sync.RWMutex

	// Configuration
	recoveryTimeout     time.Duration
	maxRecoveryAttempts int
	checkInterval       time.Duration

	// Metrics
	totalRecoveries      int64
	successfulRecoveries int64
	failedRecoveries     int64

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// RecoveryTask represents a recovery task
type RecoveryTask struct {
	ExecutionID      string                  `json:"execution_id"`
	Checkpoint       *persistence.Checkpoint `json:"checkpoint"`
	RecoveryStrategy RecoveryStrategy        `json:"recovery_strategy"`
	StartedAt        time.Time               `json:"started_at"`
	Attempts         int                     `json:"attempts"`
	LastError        error                   `json:"last_error,omitempty"`
	Status           RecoveryStatus          `json:"status"`
}

// RecoveryStatus represents the status of a recovery task
type RecoveryStatus string

const (
	RecoveryStatusPending    RecoveryStatus = "pending"
	RecoveryStatusInProgress RecoveryStatus = "in_progress"
	RecoveryStatusCompleted  RecoveryStatus = "completed"
	RecoveryStatusFailed     RecoveryStatus = "failed"
)

// RecoveryStrategy defines how to recover an execution
type RecoveryStrategy string

const (
	RecoveryStrategyResume   RecoveryStrategy = "resume"   // Resume from checkpoint
	RecoveryStrategyRestart  RecoveryStrategy = "restart"  // Restart from beginning
	RecoveryStrategyRollback RecoveryStrategy = "rollback" // Rollback to previous checkpoint
	RecoveryStrategySkip     RecoveryStrategy = "skip"     // Skip failed node
)

// ManagerConfig contains configuration for the recovery manager
type ManagerConfig struct {
	RecoveryTimeout     time.Duration
	MaxRecoveryAttempts int
	CheckInterval       time.Duration
}

// NewManager creates a new recovery manager
func NewManager(
	store *persistence.Store,
	orchestrator *orchestrator.Orchestrator,
	eventBus events.EventBus,
	config ManagerConfig,
	logger logger.Logger,
) *Manager {
	if config.RecoveryTimeout == 0 {
		config.RecoveryTimeout = 10 * time.Minute
	}
	if config.MaxRecoveryAttempts == 0 {
		config.MaxRecoveryAttempts = 3
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}

	return &Manager{
		store:                store,
		orchestrator:         orchestrator,
		eventBus:             eventBus,
		logger:               logger,
		recoveringExecutions: make(map[string]*RecoveryTask),
		recoveryTimeout:      config.RecoveryTimeout,
		maxRecoveryAttempts:  config.MaxRecoveryAttempts,
		checkInterval:        config.CheckInterval,
		stopCh:               make(chan struct{}),
	}
}

// Start starts the recovery manager
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting recovery manager")

	// Subscribe to failure events
	if err := m.subscribeToEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Start recovery checker
	m.wg.Add(1)
	go m.recoveryLoop(ctx)

	// Start metrics reporter
	m.wg.Add(1)
	go m.metricsLoop(ctx)

	// Recover any pending executions from previous run
	go m.recoverPendingExecutions(ctx)

	return nil
}

// Stop stops the recovery manager
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Stopping recovery manager")

	close(m.stopCh)

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("Recovery manager stopped")
	case <-ctx.Done():
		m.logger.Warn("Recovery manager stop timeout")
	}

	return nil
}

// RecoverExecution recovers a failed execution
func (m *Manager) RecoverExecution(ctx context.Context, executionID string, strategy RecoveryStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already recovering
	if task, exists := m.recoveringExecutions[executionID]; exists {
		if task.Status == RecoveryStatusInProgress {
			return fmt.Errorf("execution %s is already being recovered", executionID)
		}
	}

	// Get latest checkpoint
	checkpoint, err := m.store.GetLatestCheckpoint(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get checkpoint: %w", err)
	}

	// Create recovery task
	task := &RecoveryTask{
		ExecutionID:      executionID,
		Checkpoint:       checkpoint,
		RecoveryStrategy: strategy,
		StartedAt:        time.Now(),
		Status:           RecoveryStatusPending,
	}

	m.recoveringExecutions[executionID] = task

	// Start recovery in background
	go m.performRecovery(ctx, task)

	m.logger.Info("Started execution recovery",
		"executionId", executionID,
		"strategy", strategy,
	)

	return nil
}

// performRecovery performs the actual recovery
func (m *Manager) performRecovery(ctx context.Context, task *RecoveryTask) {
	// Update status
	m.updateTaskStatus(task, RecoveryStatusInProgress)

	// Create timeout context
	recoveryCtx, cancel := context.WithTimeout(ctx, m.recoveryTimeout)
	defer cancel()

	var err error

	// Perform recovery based on strategy
	switch task.RecoveryStrategy {
	case RecoveryStrategyResume:
		err = m.resumeFromCheckpoint(recoveryCtx, task)
	case RecoveryStrategyRestart:
		err = m.restartExecution(recoveryCtx, task)
	case RecoveryStrategyRollback:
		err = m.rollbackToCheckpoint(recoveryCtx, task)
	case RecoveryStrategySkip:
		err = m.skipFailedNode(recoveryCtx, task)
	default:
		err = fmt.Errorf("unknown recovery strategy: %s", task.RecoveryStrategy)
	}

	// Update task based on result
	if err != nil {
		task.LastError = err
		task.Attempts++

		if task.Attempts >= m.maxRecoveryAttempts {
			m.updateTaskStatus(task, RecoveryStatusFailed)
			m.failedRecoveries++

			m.logger.Error("Recovery failed after max attempts",
				"executionId", task.ExecutionID,
				"attempts", task.Attempts,
				"error", err,
			)

			// Publish failure event
			m.publishRecoveryEvent(ctx, task, false)
		} else {
			m.updateTaskStatus(task, RecoveryStatusPending)

			// Retry after delay
			time.Sleep(time.Duration(task.Attempts) * 30 * time.Second)
			go m.performRecovery(ctx, task)
		}
	} else {
		m.updateTaskStatus(task, RecoveryStatusCompleted)
		m.successfulRecoveries++

		m.logger.Info("Recovery completed successfully",
			"executionId", task.ExecutionID,
			"strategy", task.RecoveryStrategy,
		)

		// Publish success event
		m.publishRecoveryEvent(ctx, task, true)

		// Clean up
		m.mu.Lock()
		delete(m.recoveringExecutions, task.ExecutionID)
		m.mu.Unlock()
	}
}

// resumeFromCheckpoint resumes execution from a checkpoint
func (m *Manager) resumeFromCheckpoint(ctx context.Context, task *RecoveryTask) error {
	state := &task.Checkpoint.State

	// Create execution context from checkpoint
	executionCtx := &orchestrator.ExecutionContext{
		ExecutionID: state.ExecutionID,
		Variables:   state.Variables,
		NodeOutputs: state.NodeOutputs,
		StartTime:   state.StartTime,
		Metadata:    make(map[string]string),
	}

	// Convert errors
	for _, err := range state.Errors {
		executionCtx.Errors = append(executionCtx.Errors, orchestrator.ExecutionErrorDetail{
			NodeID:    err.NodeID,
			Error:     err.Error,
			Timestamp: err.Timestamp,
			Retryable: err.Retryable,
		})
	}

	// Resume execution from pending nodes
	for _, nodeID := range state.PendingNodes {
		// This would call the orchestrator to execute the specific node
		m.logger.Info("Resuming node execution",
			"executionId", task.ExecutionID,
			"nodeId", nodeID,
		)

		// In production, would call orchestrator.ExecuteNode(ctx, nodeID, executionCtx)
	}

	return nil
}

// restartExecution restarts an execution from the beginning
func (m *Manager) restartExecution(ctx context.Context, task *RecoveryTask) error {
	state := &task.Checkpoint.State

	// Get original workflow
	// This would retrieve the workflow from the repository

	m.logger.Info("Restarting execution",
		"executionId", task.ExecutionID,
		"workflowId", state.WorkflowID,
	)

	// Start new execution with original input
	_, err := m.orchestrator.ExecuteWorkflow(ctx, state.WorkflowID, state.Context)

	return err
}

// rollbackToCheckpoint rolls back to a previous checkpoint
func (m *Manager) rollbackToCheckpoint(ctx context.Context, task *RecoveryTask) error {
	// List all checkpoints
	checkpoints, err := m.store.ListCheckpoints(ctx, task.ExecutionID)
	if err != nil {
		return fmt.Errorf("failed to list checkpoints: %w", err)
	}

	if len(checkpoints) < 2 {
		return fmt.Errorf("no previous checkpoint to rollback to")
	}

	// Use the second-latest checkpoint (first is current)
	previousCheckpoint := checkpoints[1]

	m.logger.Info("Rolling back to previous checkpoint",
		"executionId", task.ExecutionID,
		"checkpointId", previousCheckpoint.ID,
		"timestamp", previousCheckpoint.Timestamp,
	)

	// Update task with previous checkpoint
	task.Checkpoint = previousCheckpoint

	// Resume from the previous checkpoint
	return m.resumeFromCheckpoint(ctx, task)
}

// skipFailedNode skips the failed node and continues execution
func (m *Manager) skipFailedNode(ctx context.Context, task *RecoveryTask) error {
	state := &task.Checkpoint.State

	// Find the failed node
	if len(state.Errors) == 0 {
		return fmt.Errorf("no failed node to skip")
	}

	failedNodeID := state.Errors[len(state.Errors)-1].NodeID

	m.logger.Info("Skipping failed node",
		"executionId", task.ExecutionID,
		"nodeId", failedNodeID,
	)

	// Remove failed node from pending and add to completed
	newPending := []string{}
	for _, nodeID := range state.PendingNodes {
		if nodeID != failedNodeID {
			newPending = append(newPending, nodeID)
		}
	}
	state.PendingNodes = newPending
	state.CompletedNodes = append(state.CompletedNodes, failedNodeID)

	// Continue with remaining nodes
	return m.resumeFromCheckpoint(ctx, task)
}

// updateTaskStatus updates the status of a recovery task
func (m *Manager) updateTaskStatus(task *RecoveryTask, status RecoveryStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task.Status = status
	m.totalRecoveries++
}

// publishRecoveryEvent publishes a recovery event
func (m *Manager) publishRecoveryEvent(ctx context.Context, task *RecoveryTask, success bool) {
	eventType := "recovery.failed"
	if success {
		eventType = "recovery.completed"
	}

	event := events.NewEventBuilder(eventType).
		WithAggregateID(task.ExecutionID).
		WithPayload("strategy", string(task.RecoveryStrategy)).
		WithPayload("attempts", task.Attempts).
		Build()

	if task.LastError != nil {
		event.Payload["error"] = task.LastError.Error()
	}

	m.eventBus.Publish(ctx, event)
}

// recoveryLoop periodically checks for executions needing recovery
func (m *Manager) recoveryLoop(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkForFailedExecutions(ctx)
		}
	}
}

// checkForFailedExecutions checks for failed executions that need recovery
func (m *Manager) checkForFailedExecutions(ctx context.Context) {
	// This would query the database for failed executions
	// For now, just log
	m.logger.Debug("Checking for failed executions")
}

// recoverPendingExecutions recovers executions that were pending from previous run
func (m *Manager) recoverPendingExecutions(ctx context.Context) {
	// This would query for executions that were in progress when the system stopped
	m.logger.Info("Checking for pending executions from previous run")

	// In production, would:
	// 1. Query database for executions in "running" state
	// 2. Check their last checkpoint time
	// 3. If checkpoint is old, attempt recovery
}

// metricsLoop reports recovery metrics
func (m *Manager) metricsLoop(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.reportMetrics(ctx)
		}
	}
}

// reportMetrics reports recovery metrics
func (m *Manager) reportMetrics(ctx context.Context) {
	m.mu.RLock()
	activeRecoveries := len(m.recoveringExecutions)
	m.mu.RUnlock()

	metrics := RecoveryMetrics{
		TotalRecoveries:      m.totalRecoveries,
		SuccessfulRecoveries: m.successfulRecoveries,
		FailedRecoveries:     m.failedRecoveries,
		ActiveRecoveries:     activeRecoveries,
	}

	// Publish metrics event
	event := events.NewEventBuilder("recovery.metrics").
		WithPayload("metrics", metrics).
		Build()

	m.eventBus.Publish(ctx, event)

	m.logger.Info("Recovery metrics",
		"total", metrics.TotalRecoveries,
		"successful", metrics.SuccessfulRecoveries,
		"failed", metrics.FailedRecoveries,
		"active", metrics.ActiveRecoveries,
	)
}

// subscribeToEvents subscribes to relevant events
func (m *Manager) subscribeToEvents(ctx context.Context) error {
	// Subscribe to execution failure events
	if err := m.eventBus.Subscribe(events.ExecutionFailed, m.handleExecutionFailed); err != nil {
		return err
	}

	// Subscribe to node failure events
	if err := m.eventBus.Subscribe("node.execution.failed", m.handleNodeFailed); err != nil {
		return err
	}

	return nil
}

// handleExecutionFailed handles execution failure events
func (m *Manager) handleExecutionFailed(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID

	m.logger.Info("Handling execution failure",
		"executionId", executionID,
	)

	// Attempt automatic recovery
	return m.RecoverExecution(ctx, executionID, RecoveryStrategyResume)
}

// handleNodeFailed handles node failure events
func (m *Manager) handleNodeFailed(ctx context.Context, event events.Event) error {
	executionID, _ := event.Payload["executionId"].(string)
	nodeID, _ := event.Payload["nodeId"].(string)

	m.logger.Info("Handling node failure",
		"executionId", executionID,
		"nodeId", nodeID,
	)

	// Determine recovery strategy based on error
	strategy := RecoveryStrategyResume
	if retryable, ok := event.Payload["retryable"].(bool); ok && !retryable {
		strategy = RecoveryStrategySkip
	}

	return m.RecoverExecution(ctx, executionID, strategy)
}

// GetRecoveryStatus gets the status of a recovery task
func (m *Manager) GetRecoveryStatus(executionID string) (*RecoveryTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.recoveringExecutions[executionID]
	if !exists {
		return nil, fmt.Errorf("no recovery task found for execution: %s", executionID)
	}

	return task, nil
}

// ListRecoveryTasks lists all recovery tasks
func (m *Manager) ListRecoveryTasks() []*RecoveryTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*RecoveryTask, 0, len(m.recoveringExecutions))
	for _, task := range m.recoveringExecutions {
		tasks = append(tasks, task)
	}

	return tasks
}

// RecoveryMetrics contains recovery metrics
type RecoveryMetrics struct {
	TotalRecoveries      int64 `json:"totalRecoveries"`
	SuccessfulRecoveries int64 `json:"successfulRecoveries"`
	FailedRecoveries     int64 `json:"failedRecoveries"`
	ActiveRecoveries     int   `json:"activeRecoveries"`
}
