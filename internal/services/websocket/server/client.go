package server

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512KB
)

// Client represents a WebSocket client connection
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string
	rooms  map[string]bool
}

// Message represents a WebSocket message
type Message struct {
	Type    string      `json:"type"`
	Event   string      `json:"event,omitempty"`
	Room    string      `json:"room,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("WebSocket error", "error", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from the client
func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		c.hub.logger.Error("Failed to unmarshal message", "error", err)
		return
	}

	switch msg.Type {
	case "subscribe":
		c.hub.JoinRoom(c, msg.Room)
		c.sendAck("subscribed", msg.Room)

	case "unsubscribe":
		c.hub.LeaveRoom(c, msg.Room)
		c.sendAck("unsubscribed", msg.Room)

	case "ping":
		c.sendPong()

	case "auth":
		// Handle authentication
		if token, ok := msg.Payload.(string); ok {
			userID := validateToken(token)
			if userID != "" {
				c.userID = userID
				c.sendAck("authenticated", userID)
			}
		}

	default:
		c.hub.logger.Debug("Unknown message type", "type", msg.Type)
	}
}

func (c *Client) sendAck(event, data string) {
	msg := Message{
		Type:    "ack",
		Event:   event,
		Payload: data,
	}
	if data, err := json.Marshal(msg); err == nil {
		c.send <- data
	}
}

func (c *Client) sendPong() {
	msg := Message{
		Type:    "pong",
		Payload: time.Now().UnixMilli(),
	}
	if data, err := json.Marshal(msg); err == nil {
		c.send <- data
	}
}
