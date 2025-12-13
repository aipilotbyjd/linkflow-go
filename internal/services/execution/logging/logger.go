package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// ExecutionLogger handles execution logging
type ExecutionLogger struct {
	mu          sync.RWMutex
	logger      logger.Logger
	redis       *redis.Client
	eventBus    events.EventBus
	
	// Log storage
	logs        map[string][]*ExecutionLog
	maxLogsPerExecution int
	
	// WebSocket streaming
	wsConnections map[string][]*websocket.Conn
	wsUpgrader    websocket.Upgrader
	
	// Control
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// ExecutionLog represents a log entry for an execution
type ExecutionLog struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id,omitempty"`
	Level       LogLevel               `json:"level"`
	Message     string                 `json:"message"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Source      string                 `json:"source"`
	
	// Additional context
	WorkflowID  string                 `json:"workflow_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
}

// LogLevel represents the log level
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
	LogLevelFatal   LogLevel = "fatal"
)

// NewExecutionLogger creates a new execution logger
func NewExecutionLogger(redis *redis.Client, eventBus events.EventBus, logger logger.Logger) *ExecutionLogger {
	return &ExecutionLogger{
		logger:              logger,
		redis:               redis,
		eventBus:            eventBus,
		logs:                make(map[string][]*ExecutionLog),
		maxLogsPerExecution: 10000,
		wsConnections:       make(map[string][]*websocket.Conn),
		wsUpgrader:          websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // In production, implement proper CORS check
			},
		},
		stopCh:              make(chan struct{}),
	}
}

// Start starts the execution logger
func (el *ExecutionLogger) Start(ctx context.Context) error {
	el.logger.Info("Starting execution logger")
	
	// Subscribe to events
	if err := el.subscribeToEvents(ctx); err != nil {
		return err
	}
	
	// Start log processor
	el.wg.Add(2)
	go el.processLogs(ctx)
	go el.cleanupOldLogs(ctx)
	
	return nil
}

// Stop stops the execution logger
func (el *ExecutionLogger) Stop(ctx context.Context) error {
	el.logger.Info("Stopping execution logger")
	
	close(el.stopCh)
	
	// Close all WebSocket connections
	el.mu.Lock()
	for _, conns := range el.wsConnections {
		for _, conn := range conns {
			conn.Close()
		}
	}
	el.wsConnections = make(map[string][]*websocket.Conn)
	el.mu.Unlock()
	
	done := make(chan struct{})
	go func() {
		el.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		el.logger.Info("Execution logger stopped")
	case <-ctx.Done():
		el.logger.Warn("Execution logger stop timeout")
	}
	
	return nil
}

// Log logs a message for an execution
func (el *ExecutionLogger) Log(ctx context.Context, log *ExecutionLog) error {
	if log.ID == "" {
		log.ID = fmt.Sprintf("log_%d", time.Now().UnixNano())
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	
	// Store in memory
	el.mu.Lock()
	if _, exists := el.logs[log.ExecutionID]; !exists {
		el.logs[log.ExecutionID] = make([]*ExecutionLog, 0)
	}
	
	// Limit logs per execution
	if len(el.logs[log.ExecutionID]) >= el.maxLogsPerExecution {
		// Remove oldest log
		el.logs[log.ExecutionID] = el.logs[log.ExecutionID][1:]
	}
	
	el.logs[log.ExecutionID] = append(el.logs[log.ExecutionID], log)
	el.mu.Unlock()
	
	// Store in Redis for persistence
	if err := el.persistLog(ctx, log); err != nil {
		el.logger.Error("Failed to persist log", "error", err)
	}
	
	// Stream to WebSocket connections
	el.streamLog(log)
	
	// Publish log event
	event := events.NewEventBuilder("execution.log").
		WithAggregateID(log.ExecutionID).
		WithPayload("log", log).
		Build()
	
	el.eventBus.Publish(ctx, event)
	
	return nil
}

// LogInfo logs an info message
func (el *ExecutionLogger) LogInfo(ctx context.Context, executionID, message string, data map[string]interface{}) {
	el.Log(ctx, &ExecutionLog{
		ExecutionID: executionID,
		Level:       LogLevelInfo,
		Message:     message,
		Data:        data,
		Source:      "system",
	})
}

// LogError logs an error message
func (el *ExecutionLogger) LogError(ctx context.Context, executionID, nodeID, message string, err error) {
	data := make(map[string]interface{})
	if err != nil {
		data["error"] = err.Error()
	}
	
	el.Log(ctx, &ExecutionLog{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Level:       LogLevelError,
		Message:     message,
		Data:        data,
		Source:      "system",
	})
}

// GetLogs retrieves logs for an execution
func (el *ExecutionLogger) GetLogs(ctx context.Context, executionID string, filter LogFilter) ([]*ExecutionLog, error) {
	// Try memory first
	el.mu.RLock()
	logs, exists := el.logs[executionID]
	el.mu.RUnlock()
	
	if !exists {
		// Load from Redis
		logs, err := el.loadLogsFromRedis(ctx, executionID)
		if err != nil {
			return nil, err
		}
		
		// Cache in memory
		el.mu.Lock()
		el.logs[executionID] = logs
		el.mu.Unlock()
	}
	
	// Apply filter
	filtered := el.applyFilter(logs, filter)
	
	return filtered, nil
}

// StreamLogs streams logs for an execution via WebSocket
func (el *ExecutionLogger) StreamLogs(conn *websocket.Conn, executionID string) {
	el.mu.Lock()
	if _, exists := el.wsConnections[executionID]; !exists {
		el.wsConnections[executionID] = make([]*websocket.Conn, 0)
	}
	el.wsConnections[executionID] = append(el.wsConnections[executionID], conn)
	el.mu.Unlock()
	
	// Send existing logs
	logs, _ := el.GetLogs(context.Background(), executionID, LogFilter{})
	for _, log := range logs {
		if err := conn.WriteJSON(log); err != nil {
			el.removeConnection(executionID, conn)
			return
		}
	}
	
	// Keep connection alive and wait for new logs
	for {
		select {
		case <-el.stopCh:
			return
		default:
			// Ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				el.removeConnection(executionID, conn)
				return
			}
			time.Sleep(30 * time.Second)
		}
	}
}

// persistLog persists a log to Redis
func (el *ExecutionLogger) persistLog(ctx context.Context, log *ExecutionLog) error {
	key := fmt.Sprintf("logs:execution:%s", log.ExecutionID)
	
	data, err := json.Marshal(log)
	if err != nil {
		return err
	}
	
	// Use Redis list to maintain order
	if err := el.redis.RPush(ctx, key, data).Err(); err != nil {
		return err
	}
	
	// Set TTL
	el.redis.Expire(ctx, key, 7*24*time.Hour)
	
	// Trim list to max size
	el.redis.LTrim(ctx, key, int64(-el.maxLogsPerExecution), -1)
	
	return nil
}

// loadLogsFromRedis loads logs from Redis
func (el *ExecutionLogger) loadLogsFromRedis(ctx context.Context, executionID string) ([]*ExecutionLog, error) {
	key := fmt.Sprintf("logs:execution:%s", executionID)
	
	// Get all logs
	data, err := el.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	
	logs := make([]*ExecutionLog, 0, len(data))
	for _, item := range data {
		var log ExecutionLog
		if err := json.Unmarshal([]byte(item), &log); err != nil {
			el.logger.Error("Failed to unmarshal log", "error", err)
			continue
		}
		logs = append(logs, &log)
	}
	
	return logs, nil
}

// streamLog streams a log to WebSocket connections
func (el *ExecutionLogger) streamLog(log *ExecutionLog) {
	el.mu.RLock()
	connections, exists := el.wsConnections[log.ExecutionID]
	el.mu.RUnlock()
	
	if !exists {
		return
	}
	
	// Send to all connections
	for _, conn := range connections {
		if err := conn.WriteJSON(log); err != nil {
			el.removeConnection(log.ExecutionID, conn)
		}
	}
}

// removeConnection removes a WebSocket connection
func (el *ExecutionLogger) removeConnection(executionID string, conn *websocket.Conn) {
	el.mu.Lock()
	defer el.mu.Unlock()
	
	connections, exists := el.wsConnections[executionID]
	if !exists {
		return
	}
	
	// Remove the connection
	newConnections := make([]*websocket.Conn, 0)
	for _, c := range connections {
		if c != conn {
			newConnections = append(newConnections, c)
		}
	}
	
	if len(newConnections) == 0 {
		delete(el.wsConnections, executionID)
	} else {
		el.wsConnections[executionID] = newConnections
	}
	
	conn.Close()
}

// applyFilter applies a filter to logs
func (el *ExecutionLogger) applyFilter(logs []*ExecutionLog, filter LogFilter) []*ExecutionLog {
	filtered := make([]*ExecutionLog, 0)
	
	for _, log := range logs {
		// Filter by level
		if filter.Level != "" && log.Level != filter.Level {
			continue
		}
		
		// Filter by node ID
		if filter.NodeID != "" && log.NodeID != filter.NodeID {
			continue
		}
		
		// Filter by time range
		if !filter.StartTime.IsZero() && log.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && log.Timestamp.After(filter.EndTime) {
			continue
		}
		
		// Filter by search term
		if filter.Search != "" {
			// Simple contains search
			if !contains(log.Message, filter.Search) {
				continue
			}
		}
		
		filtered = append(filtered, log)
	}
	
	// Apply limit
	if filter.Limit > 0 && len(filtered) > filter.Limit {
		filtered = filtered[len(filtered)-filter.Limit:]
	}
	
	return filtered
}

// processLogs processes logs in the background
func (el *ExecutionLogger) processLogs(ctx context.Context) {
	defer el.wg.Done()
	
	// This would implement log aggregation, analysis, etc.
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-el.stopCh:
			return
		case <-ticker.C:
			// Periodic log processing
			el.logger.Debug("Processing execution logs")
		}
	}
}

// cleanupOldLogs cleans up old logs
func (el *ExecutionLogger) cleanupOldLogs(ctx context.Context) {
	defer el.wg.Done()
	
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-el.stopCh:
			return
		case <-ticker.C:
			el.mu.Lock()
			
			// Remove old logs from memory
			for executionID, logs := range el.logs {
				if len(logs) > 0 {
					// Check if logs are older than 24 hours
					if time.Since(logs[len(logs)-1].Timestamp) > 24*time.Hour {
						delete(el.logs, executionID)
					}
				}
			}
			
			el.mu.Unlock()
		}
	}
}

// subscribeToEvents subscribes to relevant events
func (el *ExecutionLogger) subscribeToEvents(ctx context.Context) error {
	// Subscribe to execution events for automatic logging
	events := map[string]events.HandlerFunc{
		events.ExecutionStarted:       el.handleExecutionStarted,
		events.ExecutionCompleted:     el.handleExecutionCompleted,
		events.ExecutionFailed:        el.handleExecutionFailed,
		events.NodeExecutionStarted:   el.handleNodeExecutionStarted,
		events.NodeExecutionCompleted: el.handleNodeExecutionCompleted,
	}
	
	for eventType, handler := range events {
		if err := el.eventBus.Subscribe(eventType, handler); err != nil {
			return err
		}
	}
	
	return nil
}

// Event handlers

func (el *ExecutionLogger) handleExecutionStarted(ctx context.Context, event events.Event) error {
	el.LogInfo(ctx, event.AggregateID, "Execution started", map[string]interface{}{
		"workflowId": event.Payload["workflowId"],
	})
	return nil
}

func (el *ExecutionLogger) handleExecutionCompleted(ctx context.Context, event events.Event) error {
	el.LogInfo(ctx, event.AggregateID, "Execution completed", map[string]interface{}{
		"duration": event.Payload["duration"],
	})
	return nil
}

func (el *ExecutionLogger) handleExecutionFailed(ctx context.Context, event events.Event) error {
	el.LogError(ctx, event.AggregateID, "", "Execution failed", fmt.Errorf("%v", event.Payload["error"]))
	return nil
}

func (el *ExecutionLogger) handleNodeExecutionStarted(ctx context.Context, event events.Event) error {
	executionID, _ := event.Payload["executionId"].(string)
	nodeID, _ := event.Payload["nodeId"].(string)
	
	el.LogInfo(ctx, executionID, fmt.Sprintf("Node %s started", nodeID), map[string]interface{}{
		"nodeType": event.Payload["nodeType"],
	})
	return nil
}

func (el *ExecutionLogger) handleNodeExecutionCompleted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	nodeID, _ := event.Payload["nodeId"].(string)
	status, _ := event.Payload["status"].(string)
	
	if status == string(workflow.NodeExecutionCompleted) {
		el.LogInfo(ctx, executionID, fmt.Sprintf("Node %s completed", nodeID), nil)
	} else {
		el.LogError(ctx, executionID, nodeID, fmt.Sprintf("Node %s failed", nodeID), nil)
	}
	
	return nil
}

// LogFilter represents filter criteria for logs
type LogFilter struct {
	Level     LogLevel  `json:"level,omitempty"`
	NodeID    string    `json:"node_id,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Search    string    `json:"search,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || s[0:len(substr)] == substr || contains(s[1:], substr))
}
