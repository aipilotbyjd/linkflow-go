package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/linkflow-go/pkg/logger"
)

type Hub struct {
	clients    map[string]*Client
	rooms      map[string]map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	logger     logger.Logger
	mu         sync.RWMutex
}

type Client struct {
	ID       string
	UserID   string
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan Message
	Rooms    map[string]bool
	mu       sync.RWMutex
}

type Message struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Room      string                 `json:"room,omitempty"`
	UserID    string                 `json:"userId,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second
	
	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second
	
	// Send pings to peer with this period
	pingPeriod = (pongWait * 9) / 10
	
	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512KB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// In production, implement proper origin checking
		return true
	},
}

func NewHub(logger logger.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan Message, 1000),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
			
		case client := <-h.register:
			h.registerClient(client)
			
		case client := <-h.unregister:
			h.unregisterClient(client)
			
		case message := <-h.broadcast:
			h.broadcastMessage(message)
			
		case <-ticker.C:
			h.logStats()
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.clients[client.ID] = client
	
	h.logger.Info("Client registered", 
		"clientId", client.ID,
		"userId", client.UserID,
	)
	
	// Send welcome message
	welcome := Message{
		Type: "connected",
		Data: map[string]interface{}{
			"clientId": client.ID,
			"message":  "Connected to LinkFlow WebSocket",
		},
		Timestamp: time.Now(),
	}
	
	select {
	case client.Send <- welcome:
	default:
		h.logger.Warn("Welcome message dropped", "clientId", client.ID)
	}
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if _, ok := h.clients[client.ID]; ok {
		delete(h.clients, client.ID)
		close(client.Send)
		
		// Remove from all rooms
		for room := range client.Rooms {
			h.leaveRoom(client, room)
		}
		
		h.logger.Info("Client unregistered", 
			"clientId", client.ID,
			"userId", client.UserID,
		)
	}
}

func (h *Hub) broadcastMessage(message Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if message.Room != "" {
		// Send to specific room
		if clients, ok := h.rooms[message.Room]; ok {
			for client := range clients {
				select {
				case client.Send <- message:
				default:
					h.logger.Warn("Message dropped",
						"clientId", client.ID,
						"room", message.Room,
					)
				}
			}
		}
	} else {
		// Broadcast to all clients
		for _, client := range h.clients {
			select {
			case client.Send <- message:
			default:
				h.logger.Warn("Broadcast dropped", "clientId", client.ID)
			}
		}
	}
}

func (h *Hub) JoinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]bool)
	}
	
	h.rooms[room][client] = true
	client.Rooms[room] = true
	
	h.logger.Info("Client joined room",
		"clientId", client.ID,
		"room", room,
	)
	
	// Notify room members
	notification := Message{
		Type: "user_joined",
		Room: room,
		Data: map[string]interface{}{
			"userId": client.UserID,
			"room":   room,
		},
		Timestamp: time.Now(),
	}
	
	h.broadcastMessage(notification)
}

func (h *Hub) leaveRoom(client *Client, room string) {
	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
		delete(client.Rooms, room)
		
		if len(clients) == 0 {
			delete(h.rooms, room)
		}
		
		h.logger.Info("Client left room",
			"clientId", client.ID,
			"room", room,
		)
		
		// Notify room members
		notification := Message{
			Type: "user_left",
			Room: room,
			Data: map[string]interface{}{
				"userId": client.UserID,
				"room":   room,
			},
			Timestamp: time.Now(),
		}
		
		h.broadcastMessage(notification)
	}
}

func (h *Hub) LeaveRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.leaveRoom(client, room)
}

func (h *Hub) SendToUser(userID string, message Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	for _, client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Send <- message:
			default:
				h.logger.Warn("User message dropped",
					"userId", userID,
					"clientId", client.ID,
				)
			}
		}
	}
}

func (h *Hub) SendToRoom(room string, message Message) {
	message.Room = room
	h.broadcast <- message
}

func (h *Hub) GetRoomClients(room string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	var userIDs []string
	if clients, ok := h.rooms[room]; ok {
		for client := range clients {
			userIDs = append(userIDs, client.UserID)
		}
	}
	
	return userIDs
}

func (h *Hub) GetOnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	userMap := make(map[string]bool)
	for _, client := range h.clients {
		userMap[client.UserID] = true
	}
	
	var users []string
	for userID := range userMap {
		users = append(users, userID)
	}
	
	return users
}

func (h *Hub) logStats() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	h.logger.Info("WebSocket hub stats",
		"clients", len(h.clients),
		"rooms", len(h.rooms),
		"broadcast_queue", len(h.broadcast),
	)
}

func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Close all client connections
	for _, client := range h.clients {
		close(client.Send)
	}
	
	h.logger.Info("WebSocket hub shutdown")
}

// Client methods
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()
	
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	
	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.Hub.logger.Error("WebSocket read error", "error", err)
			}
			break
		}
		
		// Handle different message types
		c.handleMessage(msg)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			if err := c.Conn.WriteJSON(message); err != nil {
				return
			}
			
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg Message) {
	msg.UserID = c.UserID
	msg.Timestamp = time.Now()
	
	switch msg.Type {
	case "join_room":
		if room, ok := msg.Data["room"].(string); ok {
			c.Hub.JoinRoom(c, room)
		}
		
	case "leave_room":
		if room, ok := msg.Data["room"].(string); ok {
			c.Hub.LeaveRoom(c, room)
		}
		
	case "broadcast":
		c.Hub.broadcast <- msg
		
	case "room_message":
		if room, ok := msg.Data["room"].(string); ok {
			msg.Room = room
			c.Hub.broadcast <- msg
		}
		
	case "private_message":
		if targetUserID, ok := msg.Data["targetUserId"].(string); ok {
			c.Hub.SendToUser(targetUserID, msg)
		}
		
	default:
		// Handle custom message types
		c.Hub.logger.Debug("Unknown message type", "type", msg.Type)
	}
}
