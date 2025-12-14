package streaming

import (
	"context"
	"encoding/json"
	"time"
)

// Message represents a streaming message
type Message struct {
	ID        string                 `json:"id"`
	Topic     string                 `json:"topic"`
	Key       string                 `json:"key"`
	Value     []byte                 `json:"value"`
	Headers   map[string]string      `json:"headers"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// Producer interface for publishing messages
type Producer interface {
	// Publish publishes a message to a topic
	Publish(ctx context.Context, topic string, message *Message) error

	// PublishBatch publishes multiple messages
	PublishBatch(ctx context.Context, topic string, messages []*Message) error

	// Close closes the producer
	Close() error
}

// Consumer interface for consuming messages
type Consumer interface {
	// Subscribe subscribes to topics
	Subscribe(ctx context.Context, topics []string) error

	// Consume consumes messages from subscribed topics
	Consume(ctx context.Context) (<-chan *Message, error)

	// Commit commits the offset for a message
	Commit(ctx context.Context, message *Message) error

	// Close closes the consumer
	Close() error
}

// ConsumerGroup interface for consumer group management
type ConsumerGroup interface {
	Consumer

	// GroupID returns the consumer group ID
	GroupID() string

	// Members returns the current group members
	Members(ctx context.Context) ([]string, error)
}

// Stream interface for stream processing
type Stream interface {
	// Process processes messages with a handler
	Process(ctx context.Context, handler MessageHandler) error

	// Filter filters messages based on a predicate
	Filter(predicate func(*Message) bool) Stream

	// Map transforms messages
	Map(transformer func(*Message) *Message) Stream

	// Sink sends processed messages to a destination
	Sink(producer Producer, topic string) error
}

// MessageHandler handles incoming messages
type MessageHandler func(ctx context.Context, message *Message) error

// NewMessage creates a new message
func NewMessage(topic, key string, value interface{}) (*Message, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        generateID(),
		Topic:     topic,
		Key:       key,
		Value:     data,
		Headers:   make(map[string]string),
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}, nil
}

// Decode decodes the message value into a struct
func (m *Message) Decode(v interface{}) error {
	return json.Unmarshal(m.Value, v)
}

// SetHeader sets a header value
func (m *Message) SetHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
}

// GetHeader gets a header value
func (m *Message) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

func generateID() string {
	return time.Now().Format("20060102150405.000000")
}

// TopicConfig represents topic configuration
type TopicConfig struct {
	Name              string `json:"name"`
	Partitions        int    `json:"partitions"`
	ReplicationFactor int    `json:"replicationFactor"`
	RetentionMs       int64  `json:"retentionMs"`
	CleanupPolicy     string `json:"cleanupPolicy"` // delete, compact
}

// DefaultTopicConfig returns default topic configuration
func DefaultTopicConfig(name string) *TopicConfig {
	return &TopicConfig{
		Name:              name,
		Partitions:        3,
		ReplicationFactor: 1,
		RetentionMs:       7 * 24 * 60 * 60 * 1000, // 7 days
		CleanupPolicy:     "delete",
	}
}
