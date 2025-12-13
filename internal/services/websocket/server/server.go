package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev
	},
}

type Server struct {
	config     *config.Config
	logger     logger.Logger
	httpServer *http.Server
	hub        *Hub
	eventBus   events.EventBus
}

func New(cfg *config.Config, log logger.Logger) (*Server, error) {
	// Initialize event bus
	eventBus, err := events.NewKafkaEventBus(cfg.Kafka.ToKafkaConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create event bus: %w", err)
	}

	hub := NewHub(log)
	go hub.Run()

	router := setupRouter(hub, log)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	return &Server{
		config:     cfg,
		logger:     log,
		httpServer: httpServer,
		hub:        hub,
		eventBus:   eventBus,
	}, nil
}

func setupRouter(hub *Hub, log logger.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	// Health checks
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready", "connections": hub.ConnectionCount()})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// WebSocket endpoint
	router.GET("/ws", func(c *gin.Context) {
		handleWebSocket(hub, c.Writer, c.Request, log)
	})

	// WebSocket with authentication
	router.GET("/ws/:token", func(c *gin.Context) {
		token := c.Param("token")
		// Validate token here
		handleWebSocketWithAuth(hub, c.Writer, c.Request, token, log)
	})

	return router
}

func handleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request, log logger.Logger) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("Failed to upgrade connection", "error", err)
		return
	}

	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: "",
		rooms:  make(map[string]bool),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func handleWebSocketWithAuth(hub *Hub, w http.ResponseWriter, r *http.Request, token string, log logger.Logger) {
	// Validate token and extract user ID
	userID := validateToken(token)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("Failed to upgrade connection", "error", err)
		return
	}

	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
		rooms:  make(map[string]bool),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func validateToken(token string) string {
	// Implement JWT validation
	// Return user ID if valid, empty string if invalid
	return token // Simplified for now
}

func (s *Server) Start() error {
	// Subscribe to execution events
	go s.subscribeToEvents()

	s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Server) subscribeToEvents() {
	topics := []string{
		"execution.events",
		"workflow.events",
		"notification.events",
	}

	for _, topic := range topics {
		go func(t string) {
			err := s.eventBus.Subscribe(t, func(ctx context.Context, event events.Event) error {
				s.broadcastEvent(event)
				return nil
			})
			if err != nil {
				s.logger.Error("Failed to subscribe to topic", "topic", t, "error", err)
			}
		}(topic)
	}
}

func (s *Server) broadcastEvent(event events.Event) {
	msg := Message{
		Type:    "event",
		Event:   event.Type,
		Payload: event.Payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("Failed to marshal event", "error", err)
		return
	}

	// Broadcast to relevant rooms based on event type
	if userID, ok := event.Payload["userId"].(string); ok {
		s.hub.SendToUser(userID, data)
	}

	if workflowID, ok := event.Payload["workflowId"].(string); ok {
		s.hub.SendToRoom("workflow:"+workflowID, data)
	}

	if executionID, ok := event.Payload["executionId"].(string); ok {
		s.hub.SendToRoom("execution:"+executionID, data)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Close all WebSocket connections
	s.hub.Close()

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	// Close event bus
	if err := s.eventBus.Close(); err != nil {
		s.logger.Error("Failed to close event bus", "error", err)
	}

	return nil
}
