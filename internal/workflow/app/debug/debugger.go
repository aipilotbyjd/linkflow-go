package debug

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// Debug modes
const (
	ModeStep       = "step"
	ModeBreakpoint = "breakpoint"
	ModeObserve    = "observe"
)

// Debug session states
const (
	StateIdle      = "idle"
	StateRunning   = "running"
	StatePaused    = "paused"
	StateCompleted = "completed"
	StateError     = "error"
)

var (
	ErrSessionNotFound      = errors.New("debug session not found")
	ErrSessionAlreadyExists = errors.New("debug session already exists")
	ErrInvalidBreakpoint    = errors.New("invalid breakpoint")
	ErrNotPaused            = errors.New("session not paused")
)

// DebugSession represents a workflow debug session
type DebugSession struct {
	ID               string                 `json:"id"`
	WorkflowID       string                 `json:"workflowId"`
	ExecutionID      string                 `json:"executionId"`
	Mode             string                 `json:"mode"`
	State            string                 `json:"state"`
	CurrentNode      string                 `json:"currentNode"`
	Breakpoints      []Breakpoint           `json:"breakpoints"`
	StepMode         bool                   `json:"stepMode"`
	Variables        map[string]interface{} `json:"variables"`
	NodeOutputs      map[string]interface{} `json:"nodeOutputs"`
	Logs             []LogEntry             `json:"logs"`
	CallStack        []StackFrame           `json:"callStack"`
	WatchExpressions []WatchExpression      `json:"watchExpressions"`
	MockData         map[string]interface{} `json:"mockData"`
	StartedAt        time.Time              `json:"startedAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
}

// Breakpoint represents a debug breakpoint
type Breakpoint struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"nodeId"`
	Condition string    `json:"condition,omitempty"`
	HitCount  int       `json:"hitCount"`
	MaxHits   int       `json:"maxHits,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

// LogEntry represents a debug log entry
type LogEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	NodeID    string                 `json:"nodeId,omitempty"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// StackFrame represents a call stack frame
type StackFrame struct {
	NodeID    string                 `json:"nodeId"`
	NodeName  string                 `json:"nodeName"`
	StartTime time.Time              `json:"startTime"`
	Variables map[string]interface{} `json:"variables"`
}

// WatchExpression represents a watched expression
type WatchExpression struct {
	ID         string      `json:"id"`
	Expression string      `json:"expression"`
	Value      interface{} `json:"value"`
	Error      string      `json:"error,omitempty"`
}

// Debugger manages workflow debugging sessions
type Debugger struct {
	sessions   map[string]*DebugSession
	mu         sync.RWMutex
	redis      *redis.Client
	logger     logger.Logger
	listeners  map[string]chan DebugEvent
	listenerMu sync.RWMutex
}

// DebugEvent represents a debug event
type DebugEvent struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"sessionId"`
	NodeID    string                 `json:"nodeId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewDebugger creates a new debugger
func NewDebugger(redis *redis.Client, logger logger.Logger) *Debugger {
	return &Debugger{
		sessions:  make(map[string]*DebugSession),
		redis:     redis,
		logger:    logger,
		listeners: make(map[string]chan DebugEvent),
	}
}

// StartSession starts a new debug session
func (d *Debugger) StartSession(ctx context.Context, workflowID string, options DebugOptions) (*DebugSession, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if session already exists
	for _, session := range d.sessions {
		if session.WorkflowID == workflowID && session.State != StateCompleted {
			return nil, ErrSessionAlreadyExists
		}
	}

	// Create new session
	session := &DebugSession{
		ID:               uuid.New().String(),
		WorkflowID:       workflowID,
		Mode:             options.Mode,
		State:            StateIdle,
		Breakpoints:      []Breakpoint{},
		StepMode:         options.StepMode,
		Variables:        make(map[string]interface{}),
		NodeOutputs:      make(map[string]interface{}),
		Logs:             []LogEntry{},
		CallStack:        []StackFrame{},
		WatchExpressions: []WatchExpression{},
		MockData:         options.MockData,
		StartedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Apply initial breakpoints
	for _, bp := range options.InitialBreakpoints {
		session.Breakpoints = append(session.Breakpoints, Breakpoint{
			ID:        uuid.New().String(),
			NodeID:    bp,
			Enabled:   true,
			CreatedAt: time.Now(),
		})
	}

	// Store session
	d.sessions[session.ID] = session

	// Store in Redis for persistence
	d.storeSession(ctx, session)

	// Send event
	d.sendEvent(DebugEvent{
		Type:      "session.started",
		SessionID: session.ID,
		Timestamp: time.Now(),
	})

	d.logger.Info("Debug session started",
		"session_id", session.ID,
		"workflow_id", workflowID,
		"mode", options.Mode)

	return session, nil
}

// GetSession retrieves a debug session
func (d *Debugger) GetSession(ctx context.Context, sessionID string) (*DebugSession, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		// Try loading from Redis
		session = d.loadSession(ctx, sessionID)
		if session == nil {
			return nil, ErrSessionNotFound
		}
		d.sessions[sessionID] = session
	}

	return session, nil
}

// SetBreakpoint sets a breakpoint in a debug session
func (d *Debugger) SetBreakpoint(ctx context.Context, sessionID, nodeID string, condition string) (*Breakpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Check if breakpoint already exists
	for i, bp := range session.Breakpoints {
		if bp.NodeID == nodeID {
			// Update existing breakpoint
			session.Breakpoints[i].Condition = condition
			session.Breakpoints[i].Enabled = true
			d.storeSession(ctx, session)
			return &session.Breakpoints[i], nil
		}
	}

	// Create new breakpoint
	breakpoint := Breakpoint{
		ID:        uuid.New().String(),
		NodeID:    nodeID,
		Condition: condition,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	session.Breakpoints = append(session.Breakpoints, breakpoint)
	session.UpdatedAt = time.Now()

	d.storeSession(ctx, session)

	// Send event
	d.sendEvent(DebugEvent{
		Type:      "breakpoint.set",
		SessionID: sessionID,
		NodeID:    nodeID,
		Timestamp: time.Now(),
	})

	return &breakpoint, nil
}

// RemoveBreakpoint removes a breakpoint
func (d *Debugger) RemoveBreakpoint(ctx context.Context, sessionID, breakpointID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	// Find and remove breakpoint
	for i, bp := range session.Breakpoints {
		if bp.ID == breakpointID {
			session.Breakpoints = append(session.Breakpoints[:i], session.Breakpoints[i+1:]...)
			session.UpdatedAt = time.Now()
			d.storeSession(ctx, session)

			// Send event
			d.sendEvent(DebugEvent{
				Type:      "breakpoint.removed",
				SessionID: sessionID,
				Data:      map[string]interface{}{"breakpointId": breakpointID},
				Timestamp: time.Now(),
			})

			return nil
		}
	}

	return ErrInvalidBreakpoint
}

// Step executes a single step in debug mode
func (d *Debugger) Step(ctx context.Context, sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	if session.State != StatePaused {
		return ErrNotPaused
	}

	// Execute single step
	session.State = StateRunning
	session.UpdatedAt = time.Now()

	// Send step event
	d.sendEvent(DebugEvent{
		Type:      "execution.step",
		SessionID: sessionID,
		NodeID:    session.CurrentNode,
		Timestamp: time.Now(),
	})

	// In real implementation, this would trigger actual node execution
	// For now, we simulate it
	go func() {
		time.Sleep(100 * time.Millisecond)
		d.mu.Lock()
		session.State = StatePaused
		d.mu.Unlock()

		d.sendEvent(DebugEvent{
			Type:      "execution.paused",
			SessionID: sessionID,
			NodeID:    session.CurrentNode,
			Timestamp: time.Now(),
		})
	}()

	return nil
}

// Continue resumes execution until next breakpoint
func (d *Debugger) Continue(ctx context.Context, sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	if session.State != StatePaused {
		return ErrNotPaused
	}

	session.State = StateRunning
	session.UpdatedAt = time.Now()

	// Send continue event
	d.sendEvent(DebugEvent{
		Type:      "execution.continued",
		SessionID: sessionID,
		Timestamp: time.Now(),
	})

	return nil
}

// Pause pauses execution
func (d *Debugger) Pause(ctx context.Context, sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	if session.State != StateRunning {
		return errors.New("session not running")
	}

	session.State = StatePaused
	session.UpdatedAt = time.Now()

	// Send pause event
	d.sendEvent(DebugEvent{
		Type:      "execution.paused",
		SessionID: sessionID,
		NodeID:    session.CurrentNode,
		Timestamp: time.Now(),
	})

	return nil
}

// InspectVariable inspects a variable value
func (d *Debugger) InspectVariable(ctx context.Context, sessionID, variableName string) (interface{}, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Check current stack frame variables
	if len(session.CallStack) > 0 {
		currentFrame := session.CallStack[len(session.CallStack)-1]
		if value, ok := currentFrame.Variables[variableName]; ok {
			return value, nil
		}
	}

	// Check session variables
	if value, ok := session.Variables[variableName]; ok {
		return value, nil
	}

	return nil, fmt.Errorf("variable '%s' not found", variableName)
}

// AddWatchExpression adds a watch expression
func (d *Debugger) AddWatchExpression(ctx context.Context, sessionID, expression string) (*WatchExpression, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	watch := WatchExpression{
		ID:         uuid.New().String(),
		Expression: expression,
	}

	// Evaluate expression
	value, err := d.evaluateExpression(session, expression)
	if err != nil {
		watch.Error = err.Error()
	} else {
		watch.Value = value
	}

	session.WatchExpressions = append(session.WatchExpressions, watch)
	session.UpdatedAt = time.Now()

	d.storeSession(ctx, session)

	return &watch, nil
}

// SetMockData sets mock data for a node
func (d *Debugger) SetMockData(ctx context.Context, sessionID, nodeID string, data interface{}) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	if session.MockData == nil {
		session.MockData = make(map[string]interface{})
	}

	session.MockData[nodeID] = data
	session.UpdatedAt = time.Now()

	d.storeSession(ctx, session)

	// Send event
	d.sendEvent(DebugEvent{
		Type:      "mock.updated",
		SessionID: sessionID,
		NodeID:    nodeID,
		Timestamp: time.Now(),
	})

	return nil
}

// GetLogs retrieves debug logs
func (d *Debugger) GetLogs(ctx context.Context, sessionID string, filter LogFilter) ([]LogEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	logs := []LogEntry{}
	for _, log := range session.Logs {
		// Apply filters
		if filter.Level != "" && log.Level != filter.Level {
			continue
		}
		if filter.NodeID != "" && log.NodeID != filter.NodeID {
			continue
		}
		if !filter.StartTime.IsZero() && log.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && log.Timestamp.After(filter.EndTime) {
			continue
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// StopSession stops a debug session
func (d *Debugger) StopSession(ctx context.Context, sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	session.State = StateCompleted
	session.UpdatedAt = time.Now()

	// Clean up session after delay
	go func() {
		time.Sleep(5 * time.Minute)
		d.mu.Lock()
		delete(d.sessions, sessionID)
		d.mu.Unlock()
		d.deleteSession(ctx, sessionID)
	}()

	// Send event
	d.sendEvent(DebugEvent{
		Type:      "session.stopped",
		SessionID: sessionID,
		Timestamp: time.Now(),
	})

	d.logger.Info("Debug session stopped", "session_id", sessionID)
	return nil
}

// Subscribe subscribes to debug events
func (d *Debugger) Subscribe(sessionID string) (<-chan DebugEvent, func()) {
	d.listenerMu.Lock()
	defer d.listenerMu.Unlock()

	ch := make(chan DebugEvent, 100)
	listenerID := uuid.New().String()
	key := fmt.Sprintf("%s:%s", sessionID, listenerID)
	d.listeners[key] = ch

	unsubscribe := func() {
		d.listenerMu.Lock()
		defer d.listenerMu.Unlock()
		delete(d.listeners, key)
		close(ch)
	}

	return ch, unsubscribe
}

// Helper methods

func (d *Debugger) storeSession(ctx context.Context, session *DebugSession) {
	key := fmt.Sprintf("debug:session:%s", session.ID)
	data, _ := json.Marshal(session)
	d.redis.Set(ctx, key, string(data), 1*time.Hour)
}

func (d *Debugger) loadSession(ctx context.Context, sessionID string) *DebugSession {
	key := fmt.Sprintf("debug:session:%s", sessionID)
	data, err := d.redis.Get(ctx, key).Result()
	if err != nil {
		return nil
	}

	var session DebugSession
	json.Unmarshal([]byte(data), &session)
	return &session
}

func (d *Debugger) deleteSession(ctx context.Context, sessionID string) {
	key := fmt.Sprintf("debug:session:%s", sessionID)
	d.redis.Del(ctx, key)
}

func (d *Debugger) sendEvent(event DebugEvent) {
	d.listenerMu.RLock()
	defer d.listenerMu.RUnlock()

	for key, ch := range d.listeners {
		if event.SessionID == "" || key[:len(event.SessionID)] == event.SessionID {
			select {
			case ch <- event:
			default:
				// Channel full, skip
			}
		}
	}
}

func (d *Debugger) evaluateExpression(session *DebugSession, expression string) (interface{}, error) {
	// Simple expression evaluation - in production, use a proper expression evaluator
	// For now, just look up variables
	if value, ok := session.Variables[expression]; ok {
		return value, nil
	}

	if value, ok := session.NodeOutputs[expression]; ok {
		return value, nil
	}

	return nil, fmt.Errorf("cannot evaluate expression: %s", expression)
}

// DebugOptions defines options for debug sessions
type DebugOptions struct {
	Mode               string
	StepMode           bool
	InitialBreakpoints []string
	MockData           map[string]interface{}
	LogLevel           string
}

// LogFilter defines filter criteria for logs
type LogFilter struct {
	Level     string
	NodeID    string
	StartTime time.Time
	EndTime   time.Time
}
