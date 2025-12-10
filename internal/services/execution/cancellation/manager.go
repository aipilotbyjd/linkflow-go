package cancellation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// Manager handles execution cancellation and timeouts
type Manager struct {
	mu              sync.RWMutex
	cancellations   map[string]*CancellationContext
	timeouts        map[string]*TimeoutContext
	eventBus        events.EventBus
	logger          logger.Logger
	
	// Metrics
	totalCancellations      int64
	successfulCancellations int64
	failedCancellations     int64
	totalTimeouts           int64
	
	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// CancellationContext represents a cancellation context for an execution
type CancellationContext struct {
	ExecutionID     string                `json:"execution_id"`
	WorkflowID      string                `json:"workflow_id"`
	CancelFunc      context.CancelFunc    `json:"-"`
	Reason          string                `json:"reason"`
	RequestedBy     string                `json:"requested_by"`
	RequestedAt     time.Time             `json:"requested_at"`
	GracePeriod     time.Duration         `json:"grace_period"`
	ForceCancel     bool                  `json:"force_cancel"`
	Status          CancellationStatus    `json:"status"`
	CompletedAt     *time.Time            `json:"completed_at,omitempty"`
	
	// Cleanup tracking
	ResourcesCleaned bool                 `json:"resources_cleaned"`
	NodesCancelled   []string             `json:"nodes_cancelled"`
}

// CancellationStatus represents the status of a cancellation
type CancellationStatus string

const (
	CancellationStatusPending    CancellationStatus = "pending"
	CancellationStatusInProgress CancellationStatus = "in_progress"
	CancellationStatusCompleted  CancellationStatus = "completed"
	CancellationStatusFailed     CancellationStatus = "failed"
)

// TimeoutContext represents a timeout context for an execution
type TimeoutContext struct {
	ExecutionID       string                `json:"execution_id"`
	GlobalTimeout     time.Duration         `json:"global_timeout"`
	NodeTimeouts      map[string]time.Duration `json:"node_timeouts"`
	Timer             *time.Timer           `json:"-"`
	NodeTimers        map[string]*time.Timer `json:"-"`
	EscalationPolicy  TimeoutEscalationPolicy `json:"escalation_policy"`
	StartedAt         time.Time             `json:"started_at"`
}

// TimeoutEscalationPolicy defines how to handle timeouts
type TimeoutEscalationPolicy struct {
	WarnThreshold    float64 `json:"warn_threshold"`    // Percentage of timeout to trigger warning
	CriticalThreshold float64 `json:"critical_threshold"` // Percentage of timeout to trigger critical alert
	AutoCancel       bool    `json:"auto_cancel"`        // Whether to auto-cancel on timeout
	RetryOnTimeout   bool    `json:"retry_on_timeout"`   // Whether to retry on timeout
}

// NewManager creates a new cancellation manager
func NewManager(eventBus events.EventBus, logger logger.Logger) *Manager {
	return &Manager{
		cancellations: make(map[string]*CancellationContext),
		timeouts:      make(map[string]*TimeoutContext),
		eventBus:      eventBus,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start starts the cancellation manager
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting cancellation manager")
	
	// Subscribe to events
	if err := m.subscribeToEvents(ctx); err != nil {
		return err
	}
	
	// Start background workers
	m.wg.Add(2)
	go m.processCancellations(ctx)
	go m.monitorTimeouts(ctx)
	
	return nil
}

// Stop stops the cancellation manager
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Stopping cancellation manager")
	
	close(m.stopCh)
	
	// Cancel all active executions
	m.mu.Lock()
	for _, cancel := range m.cancellations {
		if cancel.CancelFunc != nil {
			cancel.CancelFunc()
		}
	}
	m.mu.Unlock()
	
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		m.logger.Info("Cancellation manager stopped")
	case <-ctx.Done():
		m.logger.Warn("Cancellation manager stop timeout")
	}
	
	return nil
}

// CancelExecution cancels an execution
func (m *Manager) CancelExecution(ctx context.Context, executionID string, config CancelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if already cancelled
	if cancel, exists := m.cancellations[executionID]; exists {
		if cancel.Status == CancellationStatusCompleted {
			return fmt.Errorf("execution %s already cancelled", executionID)
		}
		if cancel.Status == CancellationStatusInProgress {
			return fmt.Errorf("execution %s cancellation in progress", executionID)
		}
	}
	
	// Create cancellation context
	cancelCtx := &CancellationContext{
		ExecutionID:  executionID,
		WorkflowID:   config.WorkflowID,
		Reason:       config.Reason,
		RequestedBy:  config.RequestedBy,
		RequestedAt:  time.Now(),
		GracePeriod:  config.GracePeriod,
		ForceCancel:  config.ForceCancel,
		Status:       CancellationStatusPending,
		NodesCancelled: []string{},
	}
	
	m.cancellations[executionID] = cancelCtx
	m.totalCancellations++
	
	// Trigger cancellation
	go m.performCancellation(ctx, cancelCtx)
	
	m.logger.Info("Execution cancellation requested",
		"executionId", executionID,
		"reason", config.Reason,
		"requestedBy", config.RequestedBy,
		"forceCancel", config.ForceCancel,
	)
	
	return nil
}

// performCancellation performs the actual cancellation
func (m *Manager) performCancellation(ctx context.Context, cancel *CancellationContext) {
	// Update status
	m.updateCancellationStatus(cancel, CancellationStatusInProgress)
	
	// Send cancellation signal
	if cancel.CancelFunc != nil {
		cancel.CancelFunc()
	}
	
	// Graceful shutdown period
	if !cancel.ForceCancel && cancel.GracePeriod > 0 {
		m.logger.Info("Waiting for graceful shutdown",
			"executionId", cancel.ExecutionID,
			"gracePeriod", cancel.GracePeriod,
		)
		
		time.Sleep(cancel.GracePeriod)
	}
	
	// Stop running nodes
	if err := m.stopRunningNodes(ctx, cancel); err != nil {
		m.logger.Error("Failed to stop running nodes",
			"executionId", cancel.ExecutionID,
			"error", err,
		)
	}
	
	// Cleanup resources
	if err := m.cleanupResources(ctx, cancel); err != nil {
		m.logger.Error("Failed to cleanup resources",
			"executionId", cancel.ExecutionID,
			"error", err,
		)
		m.updateCancellationStatus(cancel, CancellationStatusFailed)
		m.failedCancellations++
		return
	}
	
	// Update execution state
	if err := m.updateExecutionState(ctx, cancel); err != nil {
		m.logger.Error("Failed to update execution state",
			"executionId", cancel.ExecutionID,
			"error", err,
		)
	}
	
	// Publish cancellation event
	m.publishCancellationEvent(ctx, cancel)
	
	// Mark as completed
	now := time.Now()
	cancel.CompletedAt = &now
	m.updateCancellationStatus(cancel, CancellationStatusCompleted)
	m.successfulCancellations++
	
	m.logger.Info("Execution cancelled successfully",
		"executionId", cancel.ExecutionID,
	)
}

// SetTimeout sets a timeout for an execution
func (m *Manager) SetTimeout(ctx context.Context, executionID string, config TimeoutConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Create or update timeout context
	timeoutCtx := &TimeoutContext{
		ExecutionID:      executionID,
		GlobalTimeout:    config.GlobalTimeout,
		NodeTimeouts:     config.NodeTimeouts,
		EscalationPolicy: config.EscalationPolicy,
		StartedAt:        time.Now(),
		NodeTimers:       make(map[string]*time.Timer),
	}
	
	// Set global timeout timer
	if config.GlobalTimeout > 0 {
		timeoutCtx.Timer = time.AfterFunc(config.GlobalTimeout, func() {
			m.handleTimeout(executionID, "")
		})
	}
	
	// Set node timeout timers
	for nodeID, timeout := range config.NodeTimeouts {
		nodeTimer := time.AfterFunc(timeout, func() {
			m.handleTimeout(executionID, nodeID)
		})
		timeoutCtx.NodeTimers[nodeID] = nodeTimer
	}
	
	m.timeouts[executionID] = timeoutCtx
	
	m.logger.Info("Timeout set for execution",
		"executionId", executionID,
		"globalTimeout", config.GlobalTimeout,
		"nodeTimeouts", len(config.NodeTimeouts),
	)
	
	// Set warning timers if configured
	if config.EscalationPolicy.WarnThreshold > 0 {
		warnTime := time.Duration(float64(config.GlobalTimeout) * config.EscalationPolicy.WarnThreshold)
		time.AfterFunc(warnTime, func() {
			m.handleTimeoutWarning(executionID)
		})
	}
	
	return nil
}

// ClearTimeout clears a timeout for an execution
func (m *Manager) ClearTimeout(executionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if timeout, exists := m.timeouts[executionID]; exists {
		if timeout.Timer != nil {
			timeout.Timer.Stop()
		}
		
		for _, timer := range timeout.NodeTimers {
			timer.Stop()
		}
		
		delete(m.timeouts, executionID)
		
		m.logger.Info("Timeout cleared for execution", "executionId", executionID)
	}
}

// handleTimeout handles execution timeout
func (m *Manager) handleTimeout(executionID string, nodeID string) {
	m.mu.RLock()
	timeout, exists := m.timeouts[executionID]
	m.mu.RUnlock()
	
	if !exists {
		return
	}
	
	m.totalTimeouts++
	
	if nodeID != "" {
		m.logger.Warn("Node execution timed out",
			"executionId", executionID,
			"nodeId", nodeID,
		)
	} else {
		m.logger.Warn("Execution timed out",
			"executionId", executionID,
			"timeout", timeout.GlobalTimeout,
		)
	}
	
	// Check escalation policy
	if timeout.EscalationPolicy.AutoCancel {
		// Auto-cancel the execution
		config := CancelConfig{
			Reason:      "Execution timeout",
			ForceCancel: true,
		}
		
		if err := m.CancelExecution(context.Background(), executionID, config); err != nil {
			m.logger.Error("Failed to auto-cancel timed out execution", "error", err)
		}
	}
	
	if timeout.EscalationPolicy.RetryOnTimeout {
		// Trigger retry
		m.triggerTimeoutRetry(executionID, nodeID)
	}
	
	// Publish timeout event
	event := events.NewEventBuilder("execution.timeout").
		WithAggregateID(executionID).
		WithPayload("nodeId", nodeID).
		WithPayload("timeout", timeout.GlobalTimeout).
		Build()
	
	m.eventBus.Publish(context.Background(), event)
}

// handleTimeoutWarning handles timeout warning
func (m *Manager) handleTimeoutWarning(executionID string) {
	m.logger.Warn("Execution approaching timeout", "executionId", executionID)
	
	// Publish warning event
	event := events.NewEventBuilder("execution.timeout.warning").
		WithAggregateID(executionID).
		Build()
	
	m.eventBus.Publish(context.Background(), event)
}

// stopRunningNodes stops all running nodes for an execution
func (m *Manager) stopRunningNodes(ctx context.Context, cancel *CancellationContext) error {
	// This would interact with the node executor to stop nodes
	// For now, just publish stop events
	
	event := events.NewEventBuilder("nodes.stop.request").
		WithAggregateID(cancel.ExecutionID).
		Build()
	
	return m.eventBus.Publish(ctx, event)
}

// cleanupResources cleans up resources for a cancelled execution
func (m *Manager) cleanupResources(ctx context.Context, cancel *CancellationContext) error {
	// Cleanup temporary files, connections, etc.
	cancel.ResourcesCleaned = true
	
	m.logger.Info("Resources cleaned up",
		"executionId", cancel.ExecutionID,
	)
	
	return nil
}

// updateExecutionState updates the execution state after cancellation
func (m *Manager) updateExecutionState(ctx context.Context, cancel *CancellationContext) error {
	// This would update the execution state in the database
	// For now, just publish state change event
	
	event := events.NewEventBuilder(events.ExecutionStateChanged).
		WithAggregateID(cancel.ExecutionID).
		WithPayload("state", workflow.ExecutionCancelled).
		WithPayload("reason", cancel.Reason).
		Build()
	
	return m.eventBus.Publish(ctx, event)
}

// updateCancellationStatus updates the cancellation status
func (m *Manager) updateCancellationStatus(cancel *CancellationContext, status CancellationStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	cancel.Status = status
}

// publishCancellationEvent publishes a cancellation event
func (m *Manager) publishCancellationEvent(ctx context.Context, cancel *CancellationContext) {
	event := events.NewEventBuilder("execution.cancelled").
		WithAggregateID(cancel.ExecutionID).
		WithPayload("reason", cancel.Reason).
		WithPayload("requestedBy", cancel.RequestedBy).
		Build()
	
	m.eventBus.Publish(ctx, event)
}

// triggerTimeoutRetry triggers a retry after timeout
func (m *Manager) triggerTimeoutRetry(executionID string, nodeID string) {
	event := events.NewEventBuilder("timeout.retry.trigger").
		WithAggregateID(executionID).
		WithPayload("nodeId", nodeID).
		Build()
	
	m.eventBus.Publish(context.Background(), event)
}

// processCancellations processes cancellation requests
func (m *Manager) processCancellations(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Check for stale cancellations
			m.checkStaleCancellations()
		}
	}
}

// monitorTimeouts monitors execution timeouts
func (m *Manager) monitorTimeouts(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Check timeout health
			m.checkTimeoutHealth()
		}
	}
}

// checkStaleCancellations checks for stale cancellations
func (m *Manager) checkStaleCancellations() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	for executionID, cancel := range m.cancellations {
		if cancel.Status == CancellationStatusInProgress {
			// Check if cancellation is taking too long
			if now.Sub(cancel.RequestedAt) > 5*time.Minute {
				m.logger.Warn("Cancellation taking too long",
					"executionId", executionID,
					"duration", now.Sub(cancel.RequestedAt),
				)
			}
		}
	}
}

// checkTimeoutHealth checks the health of timeout monitoring
func (m *Manager) checkTimeoutHealth() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	m.logger.Debug("Timeout monitor health check",
		"activeTimeouts", len(m.timeouts),
	)
}

// subscribeToEvents subscribes to relevant events
func (m *Manager) subscribeToEvents(ctx context.Context) error {
	events := map[string]events.HandlerFunc{
		events.ExecutionStarted:   m.handleExecutionStarted,
		events.ExecutionCompleted: m.handleExecutionCompleted,
		"cancel.request":          m.handleCancelRequest,
	}
	
	for eventType, handler := range events {
		if err := m.eventBus.Subscribe(eventType, handler); err != nil {
			return err
		}
	}
	
	return nil
}

// Event handlers

func (m *Manager) handleExecutionStarted(ctx context.Context, event events.Event) error {
	// Set default timeout if configured
	return nil
}

func (m *Manager) handleExecutionCompleted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	
	// Clear timeout
	m.ClearTimeout(executionID)
	
	// Remove from cancellations
	m.mu.Lock()
	delete(m.cancellations, executionID)
	m.mu.Unlock()
	
	return nil
}

func (m *Manager) handleCancelRequest(ctx context.Context, event events.Event) error {
	executionID, _ := event.Payload["executionId"].(string)
	reason, _ := event.Payload["reason"].(string)
	
	config := CancelConfig{
		Reason: reason,
	}
	
	return m.CancelExecution(ctx, executionID, config)
}

// GetCancellationStatus gets the status of a cancellation
func (m *Manager) GetCancellationStatus(executionID string) (*CancellationContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	cancel, exists := m.cancellations[executionID]
	if !exists {
		return nil, fmt.Errorf("no cancellation found for execution: %s", executionID)
	}
	
	return cancel, nil
}

// GetMetrics returns cancellation manager metrics
func (m *Manager) GetMetrics() CancellationMetrics {
	return CancellationMetrics{
		TotalCancellations:      m.totalCancellations,
		SuccessfulCancellations: m.successfulCancellations,
		FailedCancellations:     m.failedCancellations,
		TotalTimeouts:           m.totalTimeouts,
		ActiveCancellations:     len(m.cancellations),
		ActiveTimeouts:          len(m.timeouts),
	}
}

// CancelConfig contains configuration for cancellation
type CancelConfig struct {
	WorkflowID   string
	Reason       string
	RequestedBy  string
	GracePeriod  time.Duration
	ForceCancel  bool
}

// TimeoutConfig contains configuration for timeout
type TimeoutConfig struct {
	GlobalTimeout    time.Duration
	NodeTimeouts     map[string]time.Duration
	EscalationPolicy TimeoutEscalationPolicy
}

// CancellationMetrics contains cancellation manager metrics
type CancellationMetrics struct {
	TotalCancellations      int64 `json:"total_cancellations"`
	SuccessfulCancellations int64 `json:"successful_cancellations"`
	FailedCancellations     int64 `json:"failed_cancellations"`
	TotalTimeouts           int64 `json:"total_timeouts"`
	ActiveCancellations     int   `json:"active_cancellations"`
	ActiveTimeouts          int   `json:"active_timeouts"`
}
