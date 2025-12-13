package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
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

type KafkaConfig struct {
	Brokers       []string
	Topic         string
	ConsumerGroup string
}

type KafkaEventBus struct {
	config   KafkaConfig
	writer   *kafka.Writer
	readers  map[string]*kafka.Reader
	handlers map[string]EventHandler
	logger   interface{} // Use interface to avoid circular dependency
}

func NewKafkaEventBus(config KafkaConfig) (*KafkaEventBus, error) {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      config.Brokers,
		Topic:        config.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		Async:        false,
	})

	return &KafkaEventBus{
		config:   config,
		writer:   writer,
		readers:  make(map[string]*kafka.Reader),
		handlers: make(map[string]EventHandler),
	}, nil
}

func (k *KafkaEventBus) Publish(ctx context.Context, event Event) error {
	// Ensure event has an ID
	if event.ID == "" {
		event.ID = uuid.New().String()
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

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
			{Key: "correlation-id", Value: []byte(event.Metadata.CorrelationID)},
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
		MaxWait:     1 * time.Second,
	})

	k.readers[topic] = reader
	k.handlers[topic] = handler

	// Start consuming in a goroutine
	go k.consume(reader, handler)

	return nil
}

func (k *KafkaEventBus) consume(reader *kafka.Reader, handler EventHandler) {
	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			if err == context.Canceled {
				return
			}
			// Log error and continue
			fmt.Printf("Failed to read message: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		var event Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			fmt.Printf("Failed to unmarshal event: %v\n", err)
			continue
		}

		// Handle event
		if err := handler(context.Background(), event); err != nil {
			fmt.Printf("Failed to handle event: %v\n", err)
			// Implement retry logic here if needed
		}
	}
}

func (k *KafkaEventBus) Close() error {
	// Close writer
	if err := k.writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Close all readers
	for topic, reader := range k.readers {
		if err := reader.Close(); err != nil {
			return fmt.Errorf("failed to close reader for topic %s: %w", topic, err)
		}
	}

	return nil
}

// Event builder helper
type EventBuilder struct {
	event Event
}

func NewEventBuilder(eventType string) *EventBuilder {
	return &EventBuilder{
		event: Event{
			ID:        uuid.New().String(),
			Type:      eventType,
			Timestamp: time.Now().UTC(),
			Version:   1,
			Payload:   make(map[string]interface{}),
			Metadata:  EventMetadata{},
		},
	}
}

func (b *EventBuilder) WithAggregateID(id string) *EventBuilder {
	b.event.AggregateID = id
	return b
}

func (b *EventBuilder) WithAggregateType(aggregateType string) *EventBuilder {
	b.event.AggregateType = aggregateType
	return b
}

func (b *EventBuilder) WithUserID(userID string) *EventBuilder {
	b.event.UserID = userID
	return b
}

func (b *EventBuilder) WithPayload(key string, value interface{}) *EventBuilder {
	b.event.Payload[key] = value
	return b
}

func (b *EventBuilder) WithCorrelationID(id string) *EventBuilder {
	b.event.Metadata.CorrelationID = id
	return b
}

func (b *EventBuilder) WithCausationID(id string) *EventBuilder {
	b.event.Metadata.CausationID = id
	return b
}

func (b *EventBuilder) WithTraceID(id string) *EventBuilder {
	b.event.Metadata.TraceID = id
	return b
}

func (b *EventBuilder) Build() Event {
	return b.event
}

// HandlerFunc is an alias for EventHandler for backward compatibility
type HandlerFunc = EventHandler

// Common event types
const (
	// User events
	UserRegistered = "user.registered"
	UserLoggedIn   = "user.logged_in"
	UserLoggedOut  = "user.logged_out"
	UserUpdated    = "user.updated"
	UserDeleted    = "user.deleted"

	// Workflow events
	WorkflowCreated     = "workflow.created"
	WorkflowUpdated     = "workflow.updated"
	WorkflowDeleted     = "workflow.deleted"
	WorkflowActivated   = "workflow.activated"
	WorkflowDeactivated = "workflow.deactivated"

	// Execution events
	ExecutionStarted      = "execution.started"
	ExecutionCompleted    = "execution.completed"
	ExecutionFailed       = "execution.failed"
	ExecutionCancelled    = "execution.cancelled"
	ExecutionStateChanged = "execution.state_changed"
	ExecutionQueued       = "execution.queued"

	// Node events
	NodeExecutionStarted   = "node.execution.started"
	NodeExecutionCompleted = "node.execution.completed"
	NodeExecutionFailed    = "node.execution.failed"
)
