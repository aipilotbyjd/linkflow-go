package server

import (
	"sync"

	"github.com/linkflow-go/pkg/logger"
)

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	userIndex  map[string]map[*Client]bool
	roomIndex  map[string]map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	logger     logger.Logger
	mu         sync.RWMutex
}

// NewHub creates a new Hub
func NewHub(log logger.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		userIndex:  make(map[string]map[*Client]bool),
		roomIndex:  make(map[string]map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     log,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true

	if client.userID != "" {
		if h.userIndex[client.userID] == nil {
			h.userIndex[client.userID] = make(map[*Client]bool)
		}
		h.userIndex[client.userID][client] = true
	}

	h.logger.Info("Client connected", "userID", client.userID)
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		if client.userID != "" {
			if clients, ok := h.userIndex[client.userID]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.userIndex, client.userID)
				}
			}
		}

		for room := range client.rooms {
			if clients, ok := h.roomIndex[room]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.roomIndex, room)
				}
			}
		}

		h.logger.Info("Client disconnected", "userID", client.userID)
	}
}

func (h *Hub) broadcastMessage(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// SendToUser sends a message to all connections of a specific user
func (h *Hub) SendToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.userIndex[userID]; ok {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// SendToRoom sends a message to all clients in a room
func (h *Hub) SendToRoom(room string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.roomIndex[room]; ok {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// JoinRoom adds a client to a room
func (h *Hub) JoinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.roomIndex[room] == nil {
		h.roomIndex[room] = make(map[*Client]bool)
	}
	h.roomIndex[room][client] = true
	client.rooms[room] = true
}

// LeaveRoom removes a client from a room
func (h *Hub) LeaveRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.roomIndex[room]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.roomIndex, room)
		}
	}
	delete(client.rooms, room)
}

// ConnectionCount returns the number of active connections
func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Close closes all client connections
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.send)
		client.conn.Close()
	}
}
