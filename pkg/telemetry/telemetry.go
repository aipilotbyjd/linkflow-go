package telemetry

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type Telemetry struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
}

type Config struct {
	Enabled      bool
	JaegerURL    string
	ServiceName  string
	SamplingRate float64
}

func New(cfg Config) (*Telemetry, error) {
	if !cfg.Enabled {
		return &Telemetry{
			tracer: otel.Tracer("noop"),
		}, nil
	}

	// Create Jaeger exporter
	exporter, err := jaeger.New(
		jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerURL)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("environment", "production"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SamplingRate)),
	)

	// Set global provider
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Telemetry{
		tracer:   otel.Tracer(cfg.ServiceName),
		provider: provider,
	}, nil
}

func (t *Telemetry) Close() error {
	if t.provider != nil {
		return t.provider.Shutdown(context.Background())
	}
	return nil
}

func (t *Telemetry) Tracer() trace.Tracer {
	return t.tracer
}

// StartSpan starts a new span
func (t *Telemetry) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// HTTPMiddleware creates a Gin middleware for tracing
func (t *Telemetry) HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract trace context from headers
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		
		// Start span
		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		ctx, span := t.tracer.Start(ctx, spanName,
			trace.WithAttributes(
				semconv.HTTPMethodKey.String(c.Request.Method),
				semconv.HTTPTargetKey.String(c.Request.URL.Path),
				semconv.HTTPURLKey.String(c.Request.URL.String()),
				semconv.HTTPUserAgentKey.String(c.Request.UserAgent()),
				semconv.HTTPRequestContentLengthKey.Int64(c.Request.ContentLength),
				semconv.NetHostNameKey.String(c.Request.Host),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()
		
		// Store span in context
		c.Request = c.Request.WithContext(ctx)
		
		// Process request
		c.Next()
		
		// Set response attributes
		span.SetAttributes(
			semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
			semconv.HTTPResponseContentLengthKey.Int(c.Writer.Size()),
		)
		
		// Set span status based on HTTP status
		if c.Writer.Status() >= 400 {
			span.SetStatus(trace.Status{
				Code:        trace.StatusCodeError,
				Description: fmt.Sprintf("HTTP %d", c.Writer.Status()),
			})
		}
	}
}

// NewNop creates a no-op telemetry instance
func NewNop() *Telemetry {
	return &Telemetry{
		tracer: otel.Tracer("noop"),
	}
}

// Span wraps OpenTelemetry span with helper methods
type Span struct {
	span trace.Span
}

// AddEvent adds an event to the span
func (s *Span) AddEvent(name string, attrs ...attribute.KeyValue) {
	s.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetStatus sets the span status
func (s *Span) SetStatus(code trace.StatusCode, description string) {
	s.span.SetStatus(trace.Status{
		Code:        code,
		Description: description,
	})
}

// SetAttributes sets attributes on the span
func (s *Span) SetAttributes(attrs ...attribute.KeyValue) {
	s.span.SetAttributes(attrs...)
}

// End ends the span
func (s *Span) End() {
	s.span.End()
}

// RecordError records an error on the span
func (s *Span) RecordError(err error) {
	s.span.RecordError(err)
}

// Helper functions for common attributes
func ServiceAttribute(service string) attribute.KeyValue {
	return attribute.String("service.name", service)
}

func UserIDAttribute(userID string) attribute.KeyValue {
	return attribute.String("user.id", userID)
}

func WorkflowIDAttribute(workflowID string) attribute.KeyValue {
	return attribute.String("workflow.id", workflowID)
}

func ExecutionIDAttribute(executionID string) attribute.KeyValue {
	return attribute.String("execution.id", executionID)
}

func NodeIDAttribute(nodeID string) attribute.KeyValue {
	return attribute.String("node.id", nodeID)
}

func NodeTypeAttribute(nodeType string) attribute.KeyValue {
	return attribute.String("node.type", nodeType)
}

func ErrorAttribute(err error) attribute.KeyValue {
	return attribute.String("error", err.Error())
}
