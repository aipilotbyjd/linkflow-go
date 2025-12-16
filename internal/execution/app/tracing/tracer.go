package tracing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer handles distributed tracing for executions
type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	spans    map[string]trace.Span
	mu       sync.RWMutex
	logger   logger.Logger
	eventBus events.EventBus

	// Configuration
	serviceName string
	endpoint    string
	enabled     bool
}

// TracerConfig contains configuration for the tracer
type TracerConfig struct {
	ServiceName    string
	JaegerEndpoint string
	Enabled        bool
	SampleRate     float64
}

// NewTracer creates a new tracer
func NewTracer(config TracerConfig, eventBus events.EventBus, logger logger.Logger) (*Tracer, error) {
	if config.ServiceName == "" {
		config.ServiceName = "linkflow-execution"
	}
	if config.SampleRate == 0 {
		config.SampleRate = 1.0
	}

	t := &Tracer{
		spans:       make(map[string]trace.Span),
		logger:      logger,
		eventBus:    eventBus,
		serviceName: config.ServiceName,
		endpoint:    config.JaegerEndpoint,
		enabled:     config.Enabled,
	}

	if config.Enabled {
		if err := t.initializeProvider(config); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// initializeProvider initializes the OpenTelemetry provider
func (t *Tracer) initializeProvider(config TracerConfig) error {
	// Create Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerEndpoint)))
	if err != nil {
		return fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			attribute.String("environment", "production"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	t.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(t.provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Get tracer
	t.tracer = otel.Tracer(config.ServiceName)

	t.logger.Info("Tracer initialized", "endpoint", config.JaegerEndpoint)

	return nil
}

// Start starts the tracer
func (t *Tracer) Start(ctx context.Context) error {
	if !t.enabled {
		t.logger.Info("Tracing disabled")
		return nil
	}

	t.logger.Info("Starting tracer")

	// Subscribe to events
	if err := t.subscribeToEvents(ctx); err != nil {
		return err
	}

	return nil
}

// Stop stops the tracer
func (t *Tracer) Stop(ctx context.Context) error {
	if !t.enabled {
		return nil
	}

	t.logger.Info("Stopping tracer")

	// Close all active spans
	t.mu.Lock()
	for _, span := range t.spans {
		span.End()
	}
	t.spans = make(map[string]trace.Span)
	t.mu.Unlock()

	// Shutdown provider
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}

	return nil
}

// StartExecutionSpan starts a span for an execution
func (t *Tracer) StartExecutionSpan(ctx context.Context, executionID string, workflowID string) (context.Context, trace.Span) {
	if !t.enabled || t.tracer == nil {
		return ctx, nil
	}

	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("execution:%s", executionID),
		trace.WithAttributes(
			attribute.String("execution.id", executionID),
			attribute.String("workflow.id", workflowID),
			attribute.String("span.type", "execution"),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)

	// Store span
	t.mu.Lock()
	t.spans[executionID] = span
	t.mu.Unlock()

	return ctx, span
}

// StartNodeSpan starts a span for a node execution
func (t *Tracer) StartNodeSpan(ctx context.Context, executionID string, nodeID string, nodeType string) (context.Context, trace.Span) {
	if !t.enabled || t.tracer == nil {
		return ctx, nil
	}

	// Get parent span
	parentSpan := t.getSpan(executionID)
	if parentSpan != nil {
		ctx = trace.ContextWithSpan(ctx, parentSpan)
	}

	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("node:%s", nodeID),
		trace.WithAttributes(
			attribute.String("execution.id", executionID),
			attribute.String("node.id", nodeID),
			attribute.String("node.type", nodeType),
			attribute.String("span.type", "node"),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	// Store span
	spanKey := fmt.Sprintf("%s:%s", executionID, nodeID)
	t.mu.Lock()
	t.spans[spanKey] = span
	t.mu.Unlock()

	return ctx, span
}

// EndSpan ends a span
func (t *Tracer) EndSpan(spanID string, err error) {
	if !t.enabled {
		return
	}

	span := t.getSpan(spanID)
	if span == nil {
		return
	}

	// Add error if present
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
	}

	span.End()

	// Remove from map
	t.mu.Lock()
	delete(t.spans, spanID)
	t.mu.Unlock()
}

// AddEvent adds an event to a span
func (t *Tracer) AddEvent(spanID string, name string, attrs ...attribute.KeyValue) {
	if !t.enabled {
		return
	}

	span := t.getSpan(spanID)
	if span == nil {
		return
	}

	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on a span
func (t *Tracer) SetAttributes(spanID string, attrs ...attribute.KeyValue) {
	if !t.enabled {
		return
	}

	span := t.getSpan(spanID)
	if span == nil {
		return
	}

	span.SetAttributes(attrs...)
}

// getSpan gets a span by ID
func (t *Tracer) getSpan(spanID string) trace.Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.spans[spanID]
}

// subscribeToEvents subscribes to relevant events
func (t *Tracer) subscribeToEvents(ctx context.Context) error {
	events := map[string]events.HandlerFunc{
		events.ExecutionStarted:       t.handleExecutionStarted,
		events.ExecutionCompleted:     t.handleExecutionCompleted,
		events.ExecutionFailed:        t.handleExecutionFailed,
		events.NodeExecutionStarted:   t.handleNodeExecutionStarted,
		events.NodeExecutionCompleted: t.handleNodeExecutionCompleted,
	}

	for eventType, handler := range events {
		if err := t.eventBus.Subscribe(eventType, handler); err != nil {
			return err
		}
	}

	return nil
}

// Event handlers

func (t *Tracer) handleExecutionStarted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	workflowID, _ := event.Payload["workflowId"].(string)

	t.StartExecutionSpan(ctx, executionID, workflowID)
	return nil
}

func (t *Tracer) handleExecutionCompleted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	t.EndSpan(executionID, nil)
	return nil
}

func (t *Tracer) handleExecutionFailed(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	errMsg, _ := event.Payload["error"].(string)
	t.EndSpan(executionID, fmt.Errorf("%s", errMsg))
	return nil
}

func (t *Tracer) handleNodeExecutionStarted(ctx context.Context, event events.Event) error {
	executionID, _ := event.Payload["executionId"].(string)
	nodeID, _ := event.Payload["nodeId"].(string)
	nodeType, _ := event.Payload["nodeType"].(string)

	t.StartNodeSpan(ctx, executionID, nodeID, nodeType)
	return nil
}

func (t *Tracer) handleNodeExecutionCompleted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	nodeID, _ := event.Payload["nodeId"].(string)
	status, _ := event.Payload["status"].(string)

	spanKey := fmt.Sprintf("%s:%s", executionID, nodeID)

	var err error
	if status != "completed" {
		err = fmt.Errorf("node execution failed")
	}

	t.EndSpan(spanKey, err)
	return nil
}

// ExecutionTrace represents a complete trace for an execution
type ExecutionTrace struct {
	ExecutionID string        `json:"execution_id"`
	TraceID     string        `json:"trace_id"`
	Spans       []SpanInfo    `json:"spans"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     *time.Time    `json:"end_time,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Status      string        `json:"status"`
}

// SpanInfo represents information about a span
type SpanInfo struct {
	SpanID       string                 `json:"span_id"`
	ParentSpanID string                 `json:"parent_span_id,omitempty"`
	Operation    string                 `json:"operation"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Duration     time.Duration          `json:"duration,omitempty"`
	Attributes   map[string]interface{} `json:"attributes"`
	Events       []SpanEvent            `json:"events"`
	Status       string                 `json:"status"`
}

// SpanEvent represents an event in a span
type SpanEvent struct {
	Name       string                 `json:"name"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
}
