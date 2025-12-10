package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// ExecutionState represents the state of a workflow execution
type ExecutionState string

const (
	StatePending    ExecutionState = "pending"
	StateQueued     ExecutionState = "queued"
	StateRunning    ExecutionState = "running"
	StatePaused     ExecutionState = "paused"
	StateSuccess    ExecutionState = "success"
	StateFailed     ExecutionState = "failed"
	StateCancelled  ExecutionState = "cancelled"
	StateTimeout    ExecutionState = "timeout"
)

// ExecutionEvent represents events that trigger state transitions
type ExecutionEvent string

const (
	EventStart     ExecutionEvent = "start"
	EventPause     ExecutionEvent = "pause"
	EventResume    ExecutionEvent = "resume"
	EventComplete  ExecutionEvent = "complete"
	EventFail      ExecutionEvent = "fail"
	EventCancel    ExecutionEvent = "cancel"
	EventTimeout   ExecutionEvent = "timeout"
	EventQueue     ExecutionEvent = "queue"
)

// StateTransition represents a state transition record
type StateTransition struct {
	FromState ExecutionState         `json:"from_state"`
	ToState   ExecutionState         `json:"to_state"`
	Event     ExecutionEvent         `json:"event"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ExecutionStateMachine manages execution state transitions
type ExecutionStateMachine struct {
	mu          sync.RWMutex
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	State       ExecutionState         `json:"state"`
	Context     *ExecutionContext      `json:"context"`
	History     []StateTransition      `json:"history"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	eventBus    events.EventBus
	logger      logger.Logger
	
	// State-specific data
	StartedAt   *time.Time            `json:"started_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
	PausedAt    *time.Time            `json:"paused_at,omitempty"`
	Error       *ExecutionError       `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ExecutionError represents an execution error with details
type ExecutionError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	NodeID     string                 `json:"node_id,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Retryable  bool                   `json:"retryable"`
}

// NewExecutionStateMachine creates a new state machine
func NewExecutionStateMachine(
	id string,
	workflowID string,
	context *ExecutionContext,
	eventBus events.EventBus,
	logger logger.Logger,
) *ExecutionStateMachine {
	now := time.Now()
	return &ExecutionStateMachine{
		ID:         id,
		WorkflowID: workflowID,
		State:      StatePending,
		Context:    context,
		History:    []StateTransition{},
		CreatedAt:  now,
		UpdatedAt:  now,
		eventBus:   eventBus,
		logger:     logger,
		Metadata:   make(map[string]interface{}),
	}
}

// validTransitions defines valid state transitions
var validTransitions = map[ExecutionState]map[ExecutionEvent]ExecutionState{
	StatePending: {
		EventQueue:  StateQueued,
		EventStart:  StateRunning,
		EventCancel: StateCancelled,
	},
	StateQueued: {
		EventStart:   StateRunning,
		EventCancel:  StateCancelled,
		EventTimeout: StateTimeout,
	},
	StateRunning: {
		EventPause:    StatePaused,
		EventComplete: StateSuccess,
		EventFail:     StateFailed,
		EventCancel:   StateCancelled,
		EventTimeout:  StateTimeout,
	},
	StatePaused: {
		EventResume: StateRunning,
		EventCancel: StateCancelled,
		EventTimeout: StateTimeout,
	},
	StateSuccess: {
		// Terminal state - no transitions
	},
	StateFailed: {
		// Can retry by transitioning back to pending
		EventStart: StateRunning,
	},
	StateCancelled: {
		// Terminal state - no transitions
	},
	StateTimeout: {
		// Terminal state - no transitions
	},
}

// Transition handles state transitions
func (sm *ExecutionStateMachine) Transition(ctx context.Context, event ExecutionEvent, metadata map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get current state
	currentState := sm.State

	// Check if transition is valid
	transitions, exists := validTransitions[currentState]
	if !exists {
		return fmt.Errorf("no transitions defined for state %s", currentState)
	}

	newState, valid := transitions[event]
	if !valid {
		return fmt.Errorf("invalid transition: %s -> %s (event: %s)", currentState, newState, event)
	}

	// Record transition in history
	transition := StateTransition{
		FromState: currentState,
		ToState:   newState,
		Event:     event,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
	sm.History = append(sm.History, transition)

	// Update state
	sm.State = newState
	sm.UpdatedAt = time.Now()

	// Update state-specific timestamps
	now := time.Now()
	switch newState {
	case StateRunning:
		if sm.StartedAt == nil {
			sm.StartedAt = &now
		}
	case StatePaused:
		sm.PausedAt = &now
	case StateSuccess, StateFailed, StateCancelled, StateTimeout:
		sm.CompletedAt = &now
	}

	// Handle error states
	if event == EventFail && metadata != nil {
		if errMsg, ok := metadata["error"].(string); ok {
			sm.Error = &ExecutionError{
				Code:      "EXECUTION_FAILED",
				Message:   errMsg,
				Timestamp: now,
				Details:   metadata,
				Retryable: sm.isRetryable(metadata),
			}
		}
	}

	// Publish state change event
	if sm.eventBus != nil {
		stateEvent := events.NewEventBuilder(events.ExecutionStateChanged).
			WithAggregateID(sm.ID).
			WithAggregateType("execution").
			WithPayload("workflowId", sm.WorkflowID).
			WithPayload("fromState", string(currentState)).
			WithPayload("toState", string(newState)).
			WithPayload("event", string(event)).
			WithPayload("metadata", metadata).
			Build()

		if err := sm.eventBus.Publish(ctx, stateEvent); err != nil {
			sm.logger.Error("Failed to publish state change event", 
				"error", err,
				"executionId", sm.ID,
				"transition", fmt.Sprintf("%s->%s", currentState, newState))
		}
	}

	sm.logger.Info("Execution state transitioned",
		"executionId", sm.ID,
		"from", currentState,
		"to", newState,
		"event", event)

	return nil
}

// GetState returns the current state
func (sm *ExecutionStateMachine) GetState() ExecutionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.State
}

// GetHistory returns the transition history
func (sm *ExecutionStateMachine) GetHistory() []StateTransition {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return append([]StateTransition{}, sm.History...)
}

// CanTransition checks if a transition is valid from the current state
func (sm *ExecutionStateMachine) CanTransition(event ExecutionEvent) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	transitions, exists := validTransitions[sm.State]
	if !exists {
		return false
	}

	_, valid := transitions[event]
	return valid
}

// IsTerminal checks if the current state is a terminal state
func (sm *ExecutionStateMachine) IsTerminal() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	switch sm.State {
	case StateSuccess, StateFailed, StateCancelled, StateTimeout:
		return true
	default:
		return false
	}
}

// IsRunning checks if the execution is currently running
func (sm *ExecutionStateMachine) IsRunning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.State == StateRunning
}

// IsPaused checks if the execution is paused
func (sm *ExecutionStateMachine) IsPaused() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.State == StatePaused
}

// GetDuration returns the execution duration
func (sm *ExecutionStateMachine) GetDuration() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.StartedAt == nil {
		return 0
	}

	if sm.CompletedAt != nil {
		return sm.CompletedAt.Sub(*sm.StartedAt)
	}

	return time.Since(*sm.StartedAt)
}

// GetError returns the execution error if any
func (sm *ExecutionStateMachine) GetError() *ExecutionError {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.Error
}

// UpdateContext updates the execution context
func (sm *ExecutionStateMachine) UpdateContext(updateFunc func(*ExecutionContext)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.Context != nil {
		updateFunc(sm.Context)
		sm.UpdatedAt = time.Now()
	}
}

// GetContext returns a copy of the execution context
func (sm *ExecutionStateMachine) GetContext() *ExecutionContext {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if sm.Context == nil {
		return nil
	}
	
	// Create a deep copy of the context
	ctxCopy := &ExecutionContext{
		ExecutionID: sm.Context.ExecutionID,
		Variables:   make(map[string]interface{}),
		NodeOutputs: make(map[string]interface{}),
		Errors:      append([]ExecutionErrorDetail{}, sm.Context.Errors...),
		StartTime:   sm.Context.StartTime,
		Metadata:    make(map[string]string),
	}
	
	// Copy maps
	for k, v := range sm.Context.Variables {
		ctxCopy.Variables[k] = v
	}
	for k, v := range sm.Context.NodeOutputs {
		ctxCopy.NodeOutputs[k] = v
	}
	for k, v := range sm.Context.Metadata {
		ctxCopy.Metadata[k] = v
	}
	
	return ctxCopy
}

// isRetryable determines if an error is retryable
func (sm *ExecutionStateMachine) isRetryable(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	
	// Check for explicit retryable flag
	if retryable, ok := metadata["retryable"].(bool); ok {
		return retryable
	}
	
	// Check error code for known retryable patterns
	if code, ok := metadata["code"].(string); ok {
		switch code {
		case "TIMEOUT", "RATE_LIMIT", "TEMPORARY_FAILURE":
			return true
		}
	}
	
	return false
}

// ToWorkflowExecution converts the state machine to a WorkflowExecution
func (sm *ExecutionStateMachine) ToWorkflowExecution() *workflow.WorkflowExecution {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	execution := &workflow.WorkflowExecution{
		ID:         sm.ID,
		WorkflowID: sm.WorkflowID,
		Status:     string(sm.State),
		CreatedAt:  sm.CreatedAt,
		Data:       make(map[string]interface{}),
	}
	
	if sm.StartedAt != nil {
		execution.StartedAt = *sm.StartedAt
	}
	
	if sm.CompletedAt != nil {
		execution.FinishedAt = sm.CompletedAt
		if sm.StartedAt != nil {
			execution.ExecutionTime = int64(sm.CompletedAt.Sub(*sm.StartedAt).Milliseconds())
		}
	}
	
	if sm.Error != nil {
		execution.Error = sm.Error.Message
	}
	
	if sm.Context != nil {
		execution.Data = sm.Context.Variables
	}
	
	return execution
}
