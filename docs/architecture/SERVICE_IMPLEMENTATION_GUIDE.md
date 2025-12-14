# 🔧 Service Implementation Guide

## Overview
This guide provides **concrete code examples and patterns** for implementing each microservice in the go-n8n platform. Follow these patterns for consistency and best practices across all services.

---

## 📦 Base Service Template

### Service Structure
```go
// cmd/services/{service-name}/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-n8n/internal/pkg/config"
    "github.com/go-n8n/internal/pkg/logger"
    "github.com/go-n8n/internal/pkg/telemetry"
    "github.com/go-n8n/internal/services/{service-name}/server"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        panic(err)
    }

    // Initialize logger
    log := logger.New(cfg.Logger)

    // Initialize telemetry
    tel, err := telemetry.New(cfg.Telemetry)
    if err != nil {
        log.Fatal("Failed to initialize telemetry", "error", err)
    }
    defer tel.Close()

    // Create server
    srv, err := server.New(cfg, log, tel)
    if err != nil {
        log.Fatal("Failed to create server", "error", err)
    }

    // Start server
    go func() {
        if err := srv.Start(); err != nil {
            log.Fatal("Failed to start server", "error", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit

    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Error("Server forced to shutdown", "error", err)
    }

    log.Info("Server exited")
}
```

### Server Implementation
```go
// internal/services/{service-name}/server/server.go
package server

import (
    "context"
    "fmt"
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/go-n8n/internal/pkg/config"
    "github.com/go-n8n/internal/pkg/database"
    "github.com/go-n8n/internal/pkg/events"
    "github.com/go-n8n/internal/pkg/logger"
    "github.com/go-n8n/internal/pkg/telemetry"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
    config      *config.Config
    logger      logger.Logger
    telemetry   *telemetry.Telemetry
    httpServer  *http.Server
    db          *database.DB
    eventBus    events.EventBus
    handlers    *Handlers
    repository  Repository
}

func New(cfg *config.Config, log logger.Logger, tel *telemetry.Telemetry) (*Server, error) {
    // Initialize database
    db, err := database.New(cfg.Database)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }

    // Initialize event bus
    eventBus, err := events.NewKafkaEventBus(cfg.Kafka)
    if err != nil {
        return nil, fmt.Errorf("failed to create event bus: %w", err)
    }

    // Initialize repository
    repository := NewRepository(db)

    // Initialize service
    service := NewService(repository, eventBus, log)

    // Initialize handlers
    handlers := NewHandlers(service, log, tel)

    // Setup HTTP server
    router := setupRouter(handlers, tel)
    
    httpServer := &http.Server{
        Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
        Handler: router,
    }

    return &Server{
        config:     cfg,
        logger:     log,
        telemetry:  tel,
        httpServer: httpServer,
        db:         db,
        eventBus:   eventBus,
        handlers:   handlers,
        repository: repository,
    }, nil
}

func setupRouter(h *Handlers, tel *telemetry.Telemetry) *gin.Engine {
    router := gin.New()
    
    // Middleware
    router.Use(gin.Recovery())
    router.Use(tel.HTTPMiddleware())
    router.Use(corsMiddleware())
    router.Use(rateLimitMiddleware())
    
    // Health checks
    router.GET("/health", h.Health)
    router.GET("/ready", h.Ready)
    router.GET("/metrics", gin.WrapH(promhttp.Handler()))
    
    // API routes
    v1 := router.Group("/api/v1")
    {
        // Add your service-specific routes here
        v1.GET("/resources", h.ListResources)
        v1.POST("/resources", h.CreateResource)
        v1.GET("/resources/:id", h.GetResource)
        v1.PUT("/resources/:id", h.UpdateResource)
        v1.DELETE("/resources/:id", h.DeleteResource)
    }
    
    return router
}

func (s *Server) Start() error {
    // Start event consumers
    if err := s.startEventConsumers(); err != nil {
        return fmt.Errorf("failed to start event consumers: %w", err)
    }

    // Start HTTP server
    s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
    if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return fmt.Errorf("failed to start HTTP server: %w", err)
    }
    
    return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
    s.logger.Info("Shutting down server...")
    
    // Shutdown HTTP server
    if err := s.httpServer.Shutdown(ctx); err != nil {
        return fmt.Errorf("failed to shutdown HTTP server: %w", err)
    }
    
    // Close event bus
    if err := s.eventBus.Close(); err != nil {
        return fmt.Errorf("failed to close event bus: %w", err)
    }
    
    // Close database
    if err := s.db.Close(); err != nil {
        return fmt.Errorf("failed to close database: %w", err)
    }
    
    return nil
}

func (s *Server) startEventConsumers() error {
    // Subscribe to relevant events
    return s.eventBus.Subscribe("topic.events", s.handleEvent)
}

func (s *Server) handleEvent(ctx context.Context, event events.Event) error {
    s.logger.Info("Received event", "type", event.Type, "id", event.ID)
    // Handle event based on type
    return nil
}
```

---

## 🔄 Event-Driven Communication

### Event Bus Interface
```go
// internal/pkg/events/eventbus.go
package events

import (
    "context"
    "time"
)

type Event struct {
    ID            string                 `json:"id"`
    Type          string                 `json:"type"`
    AggregateID   string                 `json:"aggregateId"`
    AggregateType string                 `json:"aggregateType"`
    Timestamp     time.Time              `json:"timestamp"`
    UserID        string                 `json:"userId"`
    Version       int                    `json:"version"`
    Payload       map[string]interface{} `json:"payload"`
    Metadata      EventMetadata          `json:"metadata"`
}

type EventMetadata struct {
    CorrelationID string `json:"correlationId"`
    CausationID   string `json:"causationId"`
    TraceID       string `json:"traceId"`
    SpanID        string `json:"spanId"`
}

type EventBus interface {
    Publish(ctx context.Context, event Event) error
    Subscribe(topic string, handler EventHandler) error
    Close() error
}

type EventHandler func(ctx context.Context, event Event) error
```

### Kafka Event Bus Implementation
```go
// internal/pkg/events/kafka.go
package events

import (
    "context"
    "encoding/json"
    
    "github.com/segmentio/kafka-go"
)

type KafkaEventBus struct {
    writer   *kafka.Writer
    readers  map[string]*kafka.Reader
    handlers map[string]EventHandler
}

func NewKafkaEventBus(config KafkaConfig) (*KafkaEventBus, error) {
    writer := kafka.NewWriter(kafka.WriterConfig{
        Brokers:      config.Brokers,
        Topic:        config.Topic,
        Balancer:     &kafka.LeastBytes{},
        BatchSize:    100,
        BatchTimeout: 10 * time.Millisecond,
    })

    return &KafkaEventBus{
        writer:   writer,
        readers:  make(map[string]*kafka.Reader),
        handlers: make(map[string]EventHandler),
    }, nil
}

func (k *KafkaEventBus) Publish(ctx context.Context, event Event) error {
    data, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }

    msg := kafka.Message{
        Key:   []byte(event.AggregateID),
        Value: data,
        Headers: []kafka.Header{
            {Key: "event-type", Value: []byte(event.Type)},
            {Key: "trace-id", Value: []byte(event.Metadata.TraceID)},
        },
    }

    return k.writer.WriteMessages(ctx, msg)
}

func (k *KafkaEventBus) Subscribe(topic string, handler EventHandler) error {
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:     k.config.Brokers,
        Topic:       topic,
        GroupID:     k.config.ConsumerGroup,
        MinBytes:    1,
        MaxBytes:    10e6,
        StartOffset: kafka.LastOffset,
    })

    k.readers[topic] = reader
    k.handlers[topic] = handler

    go k.consume(reader, handler)
    
    return nil
}

func (k *KafkaEventBus) consume(reader *kafka.Reader, handler EventHandler) {
    for {
        msg, err := reader.ReadMessage(context.Background())
        if err != nil {
            log.Error("Failed to read message", "error", err)
            continue
        }

        var event Event
        if err := json.Unmarshal(msg.Value, &event); err != nil {
            log.Error("Failed to unmarshal event", "error", err)
            continue
        }

        if err := handler(context.Background(), event); err != nil {
            log.Error("Failed to handle event", "error", err)
            // Implement retry logic here
        }
    }
}
```

---

## 🎭 CQRS Implementation

### Command Handler
```go
// internal/services/workflow/commands/create_workflow.go
package commands

import (
    "context"
    "time"
    
    "github.com/go-n8n/internal/domain/workflow"
    "github.com/go-n8n/internal/pkg/events"
)

type CreateWorkflowCommand struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Nodes       []workflow.Node        `json:"nodes"`
    Connections []workflow.Connection  `json:"connections"`
    UserID      string                 `json:"userId"`
}

type CreateWorkflowHandler struct {
    repository workflow.Repository
    eventBus   events.EventBus
}

func NewCreateWorkflowHandler(repo workflow.Repository, bus events.EventBus) *CreateWorkflowHandler {
    return &CreateWorkflowHandler{
        repository: repo,
        eventBus:   bus,
    }
}

func (h *CreateWorkflowHandler) Handle(ctx context.Context, cmd CreateWorkflowCommand) (*workflow.Workflow, error) {
    // Validate command
    if err := h.validate(cmd); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // Create workflow aggregate
    wf := &workflow.Workflow{
        ID:          cmd.ID,
        Name:        cmd.Name,
        Description: cmd.Description,
        Nodes:       cmd.Nodes,
        Connections: cmd.Connections,
        UserID:      cmd.UserID,
        Status:      workflow.StatusInactive,
        Version:     1,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    // Validate workflow structure (DAG, connections, etc.)
    if err := wf.Validate(); err != nil {
        return nil, fmt.Errorf("workflow validation failed: %w", err)
    }

    // Save to repository
    if err := h.repository.Create(ctx, wf); err != nil {
        return nil, fmt.Errorf("failed to save workflow: %w", err)
    }

    // Publish event
    event := events.Event{
        ID:            uuid.New().String(),
        Type:          "workflow.created",
        AggregateID:   wf.ID,
        AggregateType: "workflow",
        Timestamp:     time.Now(),
        UserID:        cmd.UserID,
        Version:       1,
        Payload: map[string]interface{}{
            "workflowId":  wf.ID,
            "name":        wf.Name,
            "nodeCount":   len(wf.Nodes),
            "connections": len(wf.Connections),
        },
    }

    if err := h.eventBus.Publish(ctx, event); err != nil {
        // Log error but don't fail the command
        log.Error("Failed to publish event", "error", err)
    }

    return wf, nil
}

func (h *CreateWorkflowHandler) validate(cmd CreateWorkflowCommand) error {
    if cmd.Name == "" {
        return errors.New("workflow name is required")
    }
    if len(cmd.Nodes) == 0 {
        return errors.New("workflow must have at least one node")
    }
    return nil
}
```

### Query Handler
```go
// internal/services/workflow/queries/get_workflow.go
package queries

import (
    "context"
    "time"
    
    "github.com/go-n8n/internal/domain/workflow"
)

type GetWorkflowQuery struct {
    ID     string `json:"id"`
    UserID string `json:"userId"`
}

type WorkflowDTO struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Nodes       []NodeDTO              `json:"nodes"`
    Connections []ConnectionDTO        `json:"connections"`
    Status      string                 `json:"status"`
    Statistics  WorkflowStatistics     `json:"statistics"`
    CreatedAt   time.Time              `json:"createdAt"`
    UpdatedAt   time.Time              `json:"updatedAt"`
}

type GetWorkflowHandler struct {
    readModel WorkflowReadModel
    cache     Cache
}

func (h *GetWorkflowHandler) Handle(ctx context.Context, query GetWorkflowQuery) (*WorkflowDTO, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("workflow:%s", query.ID)
    if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
        return cached.(*WorkflowDTO), nil
    }

    // Query read model
    workflow, err := h.readModel.GetByID(ctx, query.ID)
    if err != nil {
        return nil, fmt.Errorf("workflow not found: %w", err)
    }

    // Check permissions
    if workflow.UserID != query.UserID && !h.hasReadPermission(ctx, query.UserID, workflow) {
        return nil, errors.New("permission denied")
    }

    // Build DTO with additional data
    dto := h.buildDTO(workflow)

    // Enrich with statistics
    stats, _ := h.readModel.GetStatistics(ctx, query.ID)
    dto.Statistics = stats

    // Cache result
    h.cache.Set(ctx, cacheKey, dto, 5*time.Minute)

    return dto, nil
}
```

---

## 🔄 Saga Pattern Implementation

### Execution Saga
```go
// internal/services/execution/saga/execution_saga.go
package saga

import (
    "context"
    "fmt"
    
    "github.com/go-n8n/internal/pkg/saga"
)

type ExecutionSaga struct {
    orchestrator *saga.Orchestrator
}

func NewExecutionSaga() *ExecutionSaga {
    orchestrator := saga.NewOrchestrator()
    
    // Define saga steps
    orchestrator.AddStep(&saga.Step{
        Name: "ValidateWorkflow",
        Action: func(ctx context.Context, data interface{}) error {
            return validateWorkflow(ctx, data)
        },
        Compensation: func(ctx context.Context, data interface{}) error {
            return nil // No compensation needed
        },
    })
    
    orchestrator.AddStep(&saga.Step{
        Name: "AllocateResources",
        Action: func(ctx context.Context, data interface{}) error {
            return allocateResources(ctx, data)
        },
        Compensation: func(ctx context.Context, data interface{}) error {
            return releaseResources(ctx, data)
        },
    })
    
    orchestrator.AddStep(&saga.Step{
        Name: "LoadCredentials",
        Action: func(ctx context.Context, data interface{}) error {
            return loadCredentials(ctx, data)
        },
        Compensation: func(ctx context.Context, data interface{}) error {
            return clearCredentials(ctx, data)
        },
    })
    
    orchestrator.AddStep(&saga.Step{
        Name: "ExecuteWorkflow",
        Action: func(ctx context.Context, data interface{}) error {
            return executeWorkflow(ctx, data)
        },
        Compensation: func(ctx context.Context, data interface{}) error {
            return cancelExecution(ctx, data)
        },
    })
    
    orchestrator.AddStep(&saga.Step{
        Name: "StoreResults",
        Action: func(ctx context.Context, data interface{}) error {
            return storeResults(ctx, data)
        },
        Compensation: func(ctx context.Context, data interface{}) error {
            return deleteResults(ctx, data)
        },
    })
    
    orchestrator.AddStep(&saga.Step{
        Name: "SendNotifications",
        Action: func(ctx context.Context, data interface{}) error {
            return sendNotifications(ctx, data)
        },
        Compensation: func(ctx context.Context, data interface{}) error {
            return nil // Notifications don't need compensation
        },
    })
    
    return &ExecutionSaga{
        orchestrator: orchestrator,
    }
}

func (s *ExecutionSaga) Execute(ctx context.Context, data ExecutionData) error {
    return s.orchestrator.Execute(ctx, data)
}
```

---

## 🔌 Service Discovery

### Consul Integration
```go
// internal/pkg/discovery/consul.go
package discovery

import (
    "fmt"
    
    "github.com/hashicorp/consul/api"
)

type ConsulDiscovery struct {
    client *api.Client
    config *ConsulConfig
}

func NewConsulDiscovery(config *ConsulConfig) (*ConsulDiscovery, error) {
    client, err := api.NewClient(&api.Config{
        Address: config.Address,
        Token:   config.Token,
    })
    if err != nil {
        return nil, err
    }
    
    return &ConsulDiscovery{
        client: client,
        config: config,
    }, nil
}

func (c *ConsulDiscovery) Register(service ServiceRegistration) error {
    registration := &api.AgentServiceRegistration{
        ID:      service.ID,
        Name:    service.Name,
        Port:    service.Port,
        Address: service.Address,
        Tags:    service.Tags,
        Check: &api.AgentServiceCheck{
            HTTP:     fmt.Sprintf("http://%s:%d/health", service.Address, service.Port),
            Interval: "10s",
            Timeout:  "5s",
        },
    }
    
    return c.client.Agent().ServiceRegister(registration)
}

func (c *ConsulDiscovery) Discover(serviceName string) ([]ServiceInstance, error) {
    services, _, err := c.client.Health().Service(serviceName, "", true, nil)
    if err != nil {
        return nil, err
    }
    
    instances := make([]ServiceInstance, 0, len(services))
    for _, service := range services {
        instances = append(instances, ServiceInstance{
            ID:      service.Service.ID,
            Address: service.Service.Address,
            Port:    service.Service.Port,
        })
    }
    
    return instances, nil
}

func (c *ConsulDiscovery) Deregister(serviceID string) error {
    return c.client.Agent().ServiceDeregister(serviceID)
}
```

---

## 🔐 Authentication Service

### JWT Manager
```go
// internal/services/auth/jwt/manager.go
package jwt

import (
    "crypto/rsa"
    "time"
    
    "github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
    privateKey *rsa.PrivateKey
    publicKey  *rsa.PublicKey
    issuer     string
}

type Claims struct {
    jwt.RegisteredClaims
    UserID      string   `json:"userId"`
    Email       string   `json:"email"`
    Roles       []string `json:"roles"`
    Permissions []string `json:"permissions"`
}

func NewJWTManager(privateKeyPath, publicKeyPath, issuer string) (*JWTManager, error) {
    privateKey, err := loadPrivateKey(privateKeyPath)
    if err != nil {
        return nil, err
    }
    
    publicKey, err := loadPublicKey(publicKeyPath)
    if err != nil {
        return nil, err
    }
    
    return &JWTManager{
        privateKey: privateKey,
        publicKey:  publicKey,
        issuer:     issuer,
    }, nil
}

func (m *JWTManager) GenerateToken(userID, email string, roles, permissions []string) (string, error) {
    claims := Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    m.issuer,
            Subject:   userID,
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
            ID:        uuid.New().String(),
        },
        UserID:      userID,
        Email:       email,
        Roles:       roles,
        Permissions: permissions,
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(m.privateKey)
}

func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return m.publicKey, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, jwt.ErrSignatureInvalid
    }
    
    return claims, nil
}

func (m *JWTManager) GenerateRefreshToken(userID string) (string, error) {
    claims := jwt.RegisteredClaims{
        Issuer:    m.issuer,
        Subject:   userID,
        IssuedAt:  jwt.NewNumericDate(time.Now()),
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
        ID:        uuid.New().String(),
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(m.privateKey)
}
```

---

## 🔄 Circuit Breaker Pattern

### Circuit Breaker Implementation
```go
// internal/pkg/resilience/circuit_breaker.go
package resilience

import (
    "context"
    "errors"
    "sync"
    "time"
)

type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

type CircuitBreaker struct {
    mu            sync.RWMutex
    state         State
    failures      int
    successes     int
    lastFailTime  time.Time
    threshold     int
    timeout       time.Duration
    halfOpenLimit int
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        state:         StateClosed,
        threshold:     threshold,
        timeout:       timeout,
        halfOpenLimit: 3,
    }
}

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
    if !cb.canExecute() {
        return errors.New("circuit breaker is open")
    }
    
    err := fn()
    cb.recordResult(err)
    
    return err
}

func (cb *CircuitBreaker) canExecute() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()
    
    switch cb.state {
    case StateClosed:
        return true
    case StateOpen:
        if time.Since(cb.lastFailTime) > cb.timeout {
            cb.mu.RUnlock()
            cb.mu.Lock()
            cb.state = StateHalfOpen
            cb.successes = 0
            cb.mu.Unlock()
            cb.mu.RLock()
            return true
        }
        return false
    case StateHalfOpen:
        return cb.successes < cb.halfOpenLimit
    default:
        return false
    }
}

func (cb *CircuitBreaker) recordResult(err error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        
        if cb.state == StateHalfOpen {
            cb.state = StateOpen
        } else if cb.failures >= cb.threshold {
            cb.state = StateOpen
        }
    } else {
        cb.successes++
        
        if cb.state == StateHalfOpen && cb.successes >= cb.halfOpenLimit {
            cb.state = StateClosed
            cb.failures = 0
        }
    }
}
```

---

## 📊 Metrics and Monitoring

### Prometheus Metrics
```go
// internal/pkg/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
    // HTTP metrics
    HTTPRequestsTotal    *prometheus.CounterVec
    HTTPRequestDuration  *prometheus.HistogramVec
    HTTPResponseSize     *prometheus.HistogramVec
    
    // Business metrics
    WorkflowsCreated     prometheus.Counter
    WorkflowsExecuted    prometheus.Counter
    ExecutionDuration    *prometheus.HistogramVec
    ExecutionStatus      *prometheus.CounterVec
    NodeExecutions       *prometheus.CounterVec
    
    // System metrics
    DBConnections        prometheus.Gauge
    CacheHitRate         prometheus.Gauge
    QueueDepth           prometheus.Gauge
    EventsPublished      *prometheus.CounterVec
    EventsConsumed       *prometheus.CounterVec
}

func NewMetrics() *Metrics {
    return &Metrics{
        HTTPRequestsTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total number of HTTP requests",
            },
            []string{"method", "endpoint", "status"},
        ),
        
        HTTPRequestDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "http_request_duration_seconds",
                Help:    "HTTP request duration in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"method", "endpoint"},
        ),
        
        WorkflowsCreated: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "workflows_created_total",
                Help: "Total number of workflows created",
            },
        ),
        
        WorkflowsExecuted: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "workflows_executed_total",
                Help: "Total number of workflow executions",
            },
        ),
        
        ExecutionDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "execution_duration_seconds",
                Help:    "Workflow execution duration in seconds",
                Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
            },
            []string{"workflow_id", "status"},
        ),
        
        ExecutionStatus: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "execution_status_total",
                Help: "Total number of executions by status",
            },
            []string{"status"},
        ),
        
        NodeExecutions: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "node_executions_total",
                Help: "Total number of node executions",
            },
            []string{"node_type", "status"},
        ),
        
        DBConnections: promauto.NewGauge(
            prometheus.GaugeOpts{
                Name: "database_connections_active",
                Help: "Number of active database connections",
            },
        ),
        
        CacheHitRate: promauto.NewGauge(
            prometheus.GaugeOpts{
                Name: "cache_hit_rate",
                Help: "Cache hit rate percentage",
            },
        ),
        
        QueueDepth: promauto.NewGauge(
            prometheus.GaugeOpts{
                Name: "queue_depth",
                Help: "Number of messages in queue",
            },
        ),
        
        EventsPublished: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "events_published_total",
                Help: "Total number of events published",
            },
            []string{"event_type"},
        ),
        
        EventsConsumed: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "events_consumed_total",
                Help: "Total number of events consumed",
            },
            []string{"event_type", "status"},
        ),
    }
}

// Middleware for HTTP metrics
func (m *Metrics) HTTPMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        duration := time.Since(start).Seconds()
        status := strconv.Itoa(c.Writer.Status())
        
        m.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
        m.HTTPRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
        m.HTTPResponseSize.WithLabelValues(c.Request.Method, c.FullPath()).Observe(float64(c.Writer.Size()))
    }
}
```

---

## 🔒 Security Middleware

### Authentication Middleware
```go
// internal/pkg/middleware/auth.go
package middleware

import (
    "strings"
    
    "github.com/gin-gonic/gin"
    "github.com/go-n8n/internal/services/auth/jwt"
)

func AuthMiddleware(jwtManager *jwt.JWTManager) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token from header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"error": "authorization header required"})
            c.Abort()
            return
        }
        
        // Validate Bearer token format
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            c.JSON(401, gin.H{"error": "invalid authorization header format"})
            c.Abort()
            return
        }
        
        // Validate token
        claims, err := jwtManager.ValidateToken(parts[1])
        if err != nil {
            c.JSON(401, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }
        
        // Set user context
        c.Set("userId", claims.UserID)
        c.Set("email", claims.Email)
        c.Set("roles", claims.Roles)
        c.Set("permissions", claims.Permissions)
        
        c.Next()
    }
}

func RequirePermission(permission string) gin.HandlerFunc {
    return func(c *gin.Context) {
        permissions, exists := c.Get("permissions")
        if !exists {
            c.JSON(403, gin.H{"error": "no permissions found"})
            c.Abort()
            return
        }
        
        userPermissions := permissions.([]string)
        for _, p := range userPermissions {
            if p == permission || p == "*" {
                c.Next()
                return
            }
        }
        
        c.JSON(403, gin.H{"error": "insufficient permissions"})
        c.Abort()
    }
}

func RequireRole(role string) gin.HandlerFunc {
    return func(c *gin.Context) {
        roles, exists := c.Get("roles")
        if !exists {
            c.JSON(403, gin.H{"error": "no roles found"})
            c.Abort()
            return
        }
        
        userRoles := roles.([]string)
        for _, r := range userRoles {
            if r == role || r == "admin" {
                c.Next()
                return
            }
        }
        
        c.JSON(403, gin.H{"error": "insufficient role"})
        c.Abort()
    }
}
```

---

## 🏗️ Repository Pattern

### Generic Repository
```go
// internal/pkg/repository/base.go
package repository

import (
    "context"
    "fmt"
    
    "gorm.io/gorm"
)

type BaseRepository[T any] struct {
    db *gorm.DB
}

func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
    return &BaseRepository[T]{db: db}
}

func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
    return r.db.WithContext(ctx).Create(entity).Error
}

func (r *BaseRepository[T]) GetByID(ctx context.Context, id string) (*T, error) {
    var entity T
    err := r.db.WithContext(ctx).Where("id = ?", id).First(&entity).Error
    if err != nil {
        return nil, err
    }
    return &entity, nil
}

func (r *BaseRepository[T]) Update(ctx context.Context, entity *T) error {
    return r.db.WithContext(ctx).Save(entity).Error
}

func (r *BaseRepository[T]) Delete(ctx context.Context, id string) error {
    var entity T
    return r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity).Error
}

func (r *BaseRepository[T]) List(ctx context.Context, offset, limit int) ([]*T, error) {
    var entities []*T
    err := r.db.WithContext(ctx).Offset(offset).Limit(limit).Find(&entities).Error
    return entities, err
}

func (r *BaseRepository[T]) Count(ctx context.Context) (int64, error) {
    var count int64
    var entity T
    err := r.db.WithContext(ctx).Model(&entity).Count(&count).Error
    return count, err
}

// Workflow-specific repository
type WorkflowRepository struct {
    *BaseRepository[workflow.Workflow]
    db *gorm.DB
}

func NewWorkflowRepository(db *gorm.DB) *WorkflowRepository {
    return &WorkflowRepository{
        BaseRepository: NewBaseRepository[workflow.Workflow](db),
        db:            db,
    }
}

func (r *WorkflowRepository) GetByUserID(ctx context.Context, userID string) ([]*workflow.Workflow, error) {
    var workflows []*workflow.Workflow
    err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&workflows).Error
    return workflows, err
}

func (r *WorkflowRepository) GetActive(ctx context.Context) ([]*workflow.Workflow, error) {
    var workflows []*workflow.Workflow
    err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&workflows).Error
    return workflows, err
}

func (r *WorkflowRepository) UpdateStatus(ctx context.Context, id string, status string) error {
    return r.db.WithContext(ctx).
        Model(&workflow.Workflow{}).
        Where("id = ?", id).
        Update("status", status).Error
}
```

---

## 🌊 Stream Processing

### Kafka Streams
```go
// internal/pkg/streaming/processor.go
package streaming

import (
    "context"
    "encoding/json"
    
    "github.com/lovoo/goka"
)

type StreamProcessor struct {
    processor *goka.Processor
}

func NewStreamProcessor(brokers []string) (*StreamProcessor, error) {
    // Define codec
    codec := goka.CodecJSON(ExecutionEvent{})
    
    // Define processor group
    group := goka.DefineGroup("execution-processor",
        goka.Input("executions", codec, processExecution),
        goka.Output("execution-results", codec),
        goka.Persist(codec),
    )
    
    // Create processor
    processor, err := goka.NewProcessor(brokers, group)
    if err != nil {
        return nil, err
    }
    
    return &StreamProcessor{
        processor: processor,
    }, nil
}

func processExecution(ctx goka.Context, msg interface{}) {
    event := msg.(*ExecutionEvent)
    
    // Get current state
    var state ExecutionState
    if v := ctx.Value(); v != nil {
        state = v.(ExecutionState)
    }
    
    // Update state based on event
    switch event.Type {
    case "execution.started":
        state.Status = "running"
        state.StartTime = event.Timestamp
    case "execution.completed":
        state.Status = "completed"
        state.EndTime = event.Timestamp
        state.Duration = state.EndTime.Sub(state.StartTime)
    case "execution.failed":
        state.Status = "failed"
        state.Error = event.Error
    }
    
    // Store updated state
    ctx.SetValue(state)
    
    // Emit result
    ctx.Emit("execution-results", event.ExecutionID, state)
}

func (p *StreamProcessor) Start() error {
    return p.processor.Run(context.Background())
}

func (p *StreamProcessor) Stop() error {
    return p.processor.Stop()
}
```

---

## 🎯 Testing Patterns

### Integration Test Setup
```go
// internal/services/{service}/tests/integration_test.go
package tests

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/suite"
    "github.com/testcontainers/testcontainers-go"
)

type IntegrationTestSuite struct {
    suite.Suite
    postgresContainer testcontainers.Container
    redisContainer    testcontainers.Container
    kafkaContainer    testcontainers.Container
    server           *server.Server
}

func (suite *IntegrationTestSuite) SetupSuite() {
    ctx := context.Background()
    
    // Start PostgreSQL container
    postgresReq := testcontainers.ContainerRequest{
        Image:        "postgres:15-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "testdb",
        },
        WaitingFor: wait.ForListeningPort("5432/tcp"),
    }
    
    postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: postgresReq,
        Started:          true,
    })
    suite.Require().NoError(err)
    suite.postgresContainer = postgres
    
    // Start Redis container
    redisReq := testcontainers.ContainerRequest{
        Image:        "redis:7-alpine",
        ExposedPorts: []string{"6379/tcp"},
        WaitingFor:   wait.ForListeningPort("6379/tcp"),
    }
    
    redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: redisReq,
        Started:          true,
    })
    suite.Require().NoError(err)
    suite.redisContainer = redis
    
    // Get connection strings
    postgresHost, _ := postgres.Host(ctx)
    postgresPort, _ := postgres.MappedPort(ctx, "5432")
    
    redisHost, _ := redis.Host(ctx)
    redisPort, _ := redis.MappedPort(ctx, "6379")
    
    // Create test configuration
    cfg := &config.Config{
        Database: config.DatabaseConfig{
            Host:     postgresHost,
            Port:     postgresPort.Int(),
            User:     "test",
            Password: "test",
            Name:     "testdb",
        },
        Redis: config.RedisConfig{
            Host: redisHost,
            Port: redisPort.Int(),
        },
    }
    
    // Create server
    server, err := server.New(cfg, logger.NewNop(), telemetry.NewNop())
    suite.Require().NoError(err)
    suite.server = server
    
    // Start server
    go server.Start()
    time.Sleep(2 * time.Second) // Wait for server to start
}

func (suite *IntegrationTestSuite) TearDownSuite() {
    ctx := context.Background()
    suite.server.Shutdown(ctx)
    suite.postgresContainer.Terminate(ctx)
    suite.redisContainer.Terminate(ctx)
    suite.kafkaContainer.Terminate(ctx)
}

func (suite *IntegrationTestSuite) TestCreateWorkflow() {
    // Test workflow creation
    workflow := &CreateWorkflowRequest{
        Name:        "Test Workflow",
        Description: "Integration test workflow",
        Nodes: []NodeRequest{
            {
                ID:   "node1",
                Type: "http-request",
                Parameters: map[string]interface{}{
                    "url":    "https://api.example.com",
                    "method": "GET",
                },
            },
        },
    }
    
    resp, err := suite.client.CreateWorkflow(workflow)
    suite.Require().NoError(err)
    suite.Assert().NotEmpty(resp.ID)
    suite.Assert().Equal("Test Workflow", resp.Name)
}

func TestIntegrationSuite(t *testing.T) {
    suite.Run(t, new(IntegrationTestSuite))
}
```

---

## 🚀 Deployment Configuration

### Dockerfile
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build service
ARG SERVICE_NAME
RUN make build SERVICE=${SERVICE_NAME}

# Runtime stage
FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
ARG SERVICE_NAME
COPY --from=builder /app/bin/${SERVICE_NAME} /app/service

# Copy config
COPY --from=builder /app/configs /app/configs

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run service
EXPOSE 8080
CMD ["/app/service"]
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: workflow-service
  namespace: n8n-clone
spec:
  replicas: 3
  selector:
    matchLabels:
      app: workflow-service
  template:
    metadata:
      labels:
        app: workflow-service
        version: v1
    spec:
      containers:
      - name: workflow-service
        image: go-n8n/workflow-service:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: database-secret
              key: host
        - name: KAFKA_BROKERS
          value: "kafka-0:9092,kafka-1:9092,kafka-2:9092"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - workflow-service
              topologyKey: kubernetes.io/hostname
```

---

## 🎉 Conclusion

This implementation guide provides:

1. **Complete service templates** ready to copy and customize
2. **Production-ready patterns** for all major concerns
3. **Testing strategies** with real examples
4. **Deployment configurations** for Kubernetes
5. **Security implementations** following best practices

Use these patterns consistently across all services to maintain code quality and ensure the system scales smoothly to handle enterprise workloads.

**Remember:**
- Start small, iterate fast
- Test everything
- Monitor from day one
- Document as you build
- Keep services loosely coupled

**Next Steps:**
1. Copy the base service template
2. Implement your first service (auth-service recommended)
3. Add event publishing
4. Deploy to Kubernetes
5. Iterate and improve

Good luck building your world-class workflow automation platform! 🚀
