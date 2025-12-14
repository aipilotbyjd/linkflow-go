package websocket

import (
	"time"
)

// Message represents a WebSocket message
type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	Room      string                 `json:"room,omitempty"`
	UserID    string                 `json:"userId,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// MessageType represents the type of WebSocket message
type MessageType string

const (
	// Connection messages
	MessageTypeConnected    MessageType = "connected"
	MessageTypeDisconnected MessageType = "disconnected"
	MessageTypePing         MessageType = "ping"
	MessageTypePong         MessageType = "pong"

	// Room messages
	MessageTypeJoinRoom    MessageType = "join_room"
	MessageTypeLeaveRoom   MessageType = "leave_room"
	MessageTypeUserJoined  MessageType = "user_joined"
	MessageTypeUserLeft    MessageType = "user_left"
	MessageTypeRoomMessage MessageType = "room_message"

	// Broadcast messages
	MessageTypeBroadcast      MessageType = "broadcast"
	MessageTypePrivateMessage MessageType = "private_message"

	// Execution updates
	MessageTypeExecutionStarted   MessageType = "execution_started"
	MessageTypeExecutionCompleted MessageType = "execution_completed"
	MessageTypeExecutionFailed    MessageType = "execution_failed"
	MessageTypeExecutionProgress  MessageType = "execution_progress"
	MessageTypeNodeStarted        MessageType = "node_started"
	MessageTypeNodeCompleted      MessageType = "node_completed"
	MessageTypeNodeFailed         MessageType = "node_failed"

	// Workflow updates
	MessageTypeWorkflowUpdated   MessageType = "workflow_updated"
	MessageTypeWorkflowActivated MessageType = "workflow_activated"
	MessageTypeWorkflowDeleted   MessageType = "workflow_deleted"

	// Notifications
	MessageTypeNotification MessageType = "notification"
	MessageTypeAlert        MessageType = "alert"
)

// Subscription represents a client subscription to events
type Subscription struct {
	ID        string           `json:"id"`
	ClientID  string           `json:"clientId"`
	UserID    string           `json:"userId"`
	Type      SubscriptionType `json:"type"`
	Target    string           `json:"target"` // workflowId, executionId, room name, etc.
	CreatedAt time.Time        `json:"createdAt"`
}

// SubscriptionType represents what the client is subscribed to
type SubscriptionType string

const (
	SubscriptionTypeExecution SubscriptionType = "execution"
	SubscriptionTypeWorkflow  SubscriptionType = "workflow"
	SubscriptionTypeRoom      SubscriptionType = "room"
	SubscriptionTypeUser      SubscriptionType = "user"
	SubscriptionTypeGlobal    SubscriptionType = "global"
)

// ClientInfo represents connected client information
type ClientInfo struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	IPAddress    string    `json:"ipAddress"`
	UserAgent    string    `json:"userAgent"`
	Rooms        []string  `json:"rooms"`
	ConnectedAt  time.Time `json:"connectedAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
}

// RoomInfo represents a WebSocket room
type RoomInfo struct {
	Name      string                 `json:"name"`
	Type      RoomType               `json:"type"`
	Clients   []ClientInfo           `json:"clients"`
	CreatedAt time.Time              `json:"createdAt"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// RoomType represents the type of room
type RoomType string

const (
	RoomTypeExecution RoomType = "execution" // For execution updates
	RoomTypeWorkflow  RoomType = "workflow"  // For workflow collaboration
	RoomTypeTeam      RoomType = "team"      // For team communication
	RoomTypeCustom    RoomType = "custom"    // User-created rooms
)

// ExecutionUpdate represents a real-time execution update
type ExecutionUpdate struct {
	ExecutionID string                 `json:"executionId"`
	WorkflowID  string                 `json:"workflowId"`
	Status      string                 `json:"status"`
	NodeID      string                 `json:"nodeId,omitempty"`
	NodeStatus  string                 `json:"nodeStatus,omitempty"`
	Progress    float64                `json:"progress"` // 0-100
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// NewMessage creates a new WebSocket message
func NewMessage(msgType MessageType, data map[string]interface{}) *Message {
	return &Message{
		ID:        generateID(),
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewRoomMessage creates a new room message
func NewRoomMessage(room string, msgType MessageType, data map[string]interface{}) *Message {
	msg := NewMessage(msgType, data)
	msg.Room = room
	return msg
}

// NewExecutionUpdate creates a new execution update message
func NewExecutionUpdate(executionID, workflowID, status string) *ExecutionUpdate {
	return &ExecutionUpdate{
		ExecutionID: executionID,
		WorkflowID:  workflowID,
		Status:      status,
		Timestamp:   time.Now(),
	}
}

// ToMessage converts ExecutionUpdate to a Message
func (e *ExecutionUpdate) ToMessage() *Message {
	return &Message{
		ID:   generateID(),
		Type: MessageTypeExecutionProgress,
		Room: "execution:" + e.ExecutionID,
		Data: map[string]interface{}{
			"executionId": e.ExecutionID,
			"workflowId":  e.WorkflowID,
			"status":      e.Status,
			"nodeId":      e.NodeID,
			"nodeStatus":  e.NodeStatus,
			"progress":    e.Progress,
			"data":        e.Data,
			"error":       e.Error,
		},
		Timestamp: e.Timestamp,
	}
}

func generateID() string {
	return time.Now().Format("20060102150405.000000")
}
