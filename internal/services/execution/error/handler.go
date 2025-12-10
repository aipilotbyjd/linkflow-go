package error

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/internal/services/execution/retry"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// Handler handles execution errors
type Handler struct {
	mu               sync.RWMutex
	errorHandlers    map[ErrorCode]ErrorHandler
	errorClassifier  *DefaultErrorClassifier
	retryManager     *retry.Manager
	eventBus        events.EventBus
	logger          logger.Logger
	
	// Error workflow triggers
	errorWorkflows  map[ErrorCode]string
	
	// Metrics
	totalErrors     int64
	handledErrors   int64
	unhandledErrors int64
}

// ErrorHandler interface for handling specific error types
type ErrorHandler interface {
	Handle(ctx context.Context, err ExecutionError) error
	CanHandle(err ExecutionError) bool
}

// ExecutionError represents an execution error with context
type ExecutionError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	NodeID      string                 `json:"node_id,omitempty"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	WorkflowID  string                 `json:"workflow_id,omitempty"`
	Cause       error                  `json:"-"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Retryable   bool                   `json:"retryable"`
	Severity    ErrorSeverity          `json:"severity"`
}

// ErrorCode represents an error code
type ErrorCode string

const (
	ErrorCodeTimeout           ErrorCode = "TIMEOUT"
	ErrorCodeNodeFailed        ErrorCode = "NODE_FAILED"
	ErrorCodeInvalidInput      ErrorCode = "INVALID_INPUT"
	ErrorCodeResourceNotFound  ErrorCode = "RESOURCE_NOT_FOUND"
	ErrorCodePermissionDenied  ErrorCode = "PERMISSION_DENIED"
	ErrorCodeRateLimited       ErrorCode = "RATE_LIMITED"
	ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrorCodeNetworkError      ErrorCode = "NETWORK_ERROR"
	ErrorCodeScriptError       ErrorCode = "SCRIPT_ERROR"
	ErrorCodeDatabaseError     ErrorCode = "DATABASE_ERROR"
	ErrorCodeAPIError          ErrorCode = "API_ERROR"
	ErrorCodeUnknown           ErrorCode = "UNKNOWN"
)

// ErrorSeverity represents the severity of an error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// NewHandler creates a new error handler
func NewHandler(retryManager *retry.Manager, eventBus events.EventBus, logger logger.Logger) *Handler {
	handler := &Handler{
		errorHandlers:   make(map[ErrorCode]ErrorHandler),
		errorClassifier: NewDefaultErrorClassifier(),
		retryManager:    retryManager,
		eventBus:        eventBus,
		logger:          logger,
		errorWorkflows:  make(map[ErrorCode]string),
	}
	
	// Register default error handlers
	handler.registerDefaultHandlers()
	
	return handler
}

// registerDefaultHandlers registers default error handlers
func (h *Handler) registerDefaultHandlers() {
	h.RegisterHandler(ErrorCodeTimeout, &TimeoutErrorHandler{logger: h.logger})
	h.RegisterHandler(ErrorCodeRateLimited, &RateLimitErrorHandler{logger: h.logger})
	h.RegisterHandler(ErrorCodeServiceUnavailable, &ServiceErrorHandler{logger: h.logger})
	h.RegisterHandler(ErrorCodeScriptError, &ScriptErrorHandler{logger: h.logger})
	h.RegisterHandler(ErrorCodeDatabaseError, &DatabaseErrorHandler{logger: h.logger})
}

// Start starts the error handler
func (h *Handler) Start(ctx context.Context) error {
	h.logger.Info("Starting error handler")
	
	// Subscribe to error events
	if err := h.subscribeToEvents(ctx); err != nil {
		return err
	}
	
	return nil
}

// Stop stops the error handler
func (h *Handler) Stop(ctx context.Context) error {
	h.logger.Info("Stopping error handler")
	return nil
}

// RegisterHandler registers an error handler
func (h *Handler) RegisterHandler(code ErrorCode, handler ErrorHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.errorHandlers[code] = handler
	h.logger.Info("Registered error handler", "code", code)
}

// RegisterErrorWorkflow registers an error workflow trigger
func (h *Handler) RegisterErrorWorkflow(code ErrorCode, workflowID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.errorWorkflows[code] = workflowID
	h.logger.Info("Registered error workflow", "code", code, "workflowId", workflowID)
}

// HandleError handles an execution error
func (h *Handler) HandleError(ctx context.Context, err ExecutionError) error {
	h.totalErrors++
	
	// Classify error
	err.Code = h.classifyError(err.Cause)
	err.Severity = h.determineSeverity(err.Code)
	err.Retryable = h.errorClassifier.IsRetryable(err.Cause)
	
	// Log error
	h.logError(err)
	
	// Get specific handler
	handler := h.getHandler(err.Code)
	if handler != nil && handler.CanHandle(err) {
		h.handledErrors++
		if handleErr := handler.Handle(ctx, err); handleErr != nil {
			h.logger.Error("Error handler failed", "error", handleErr)
			return handleErr
		}
	} else {
		h.unhandledErrors++
		h.logger.Warn("No handler for error", "code", err.Code)
	}
	
	// Trigger error workflow if configured
	if workflowID := h.getErrorWorkflow(err.Code); workflowID != "" {
		h.triggerErrorWorkflow(ctx, workflowID, err)
	}
	
	// Publish error event
	h.publishErrorEvent(ctx, err)
	
	return nil
}

// classifyError classifies an error into an error code
func (h *Handler) classifyError(err error) ErrorCode {
	if err == nil {
		return ErrorCodeUnknown
	}
	
	errMsg := strings.ToLower(err.Error())
	
	// Check for specific error patterns
	switch {
	case strings.Contains(errMsg, "timeout"):
		return ErrorCodeTimeout
	case strings.Contains(errMsg, "rate limit"):
		return ErrorCodeRateLimited
	case strings.Contains(errMsg, "permission denied"):
		return ErrorCodePermissionDenied
	case strings.Contains(errMsg, "not found"):
		return ErrorCodeResourceNotFound
	case strings.Contains(errMsg, "invalid"):
		return ErrorCodeInvalidInput
	case strings.Contains(errMsg, "network"):
		return ErrorCodeNetworkError
	case strings.Contains(errMsg, "database"):
		return ErrorCodeDatabaseError
	case strings.Contains(errMsg, "api"):
		return ErrorCodeAPIError
	case strings.Contains(errMsg, "script"):
		return ErrorCodeScriptError
	case strings.Contains(errMsg, "service unavailable"):
		return ErrorCodeServiceUnavailable
	default:
		return ErrorCodeUnknown
	}
}

// determineSeverity determines the severity of an error
func (h *Handler) determineSeverity(code ErrorCode) ErrorSeverity {
	switch code {
	case ErrorCodeTimeout, ErrorCodeRateLimited:
		return SeverityLow
	case ErrorCodeInvalidInput, ErrorCodeResourceNotFound:
		return SeverityMedium
	case ErrorCodePermissionDenied, ErrorCodeScriptError:
		return SeverityHigh
	case ErrorCodeServiceUnavailable, ErrorCodeDatabaseError:
		return SeverityCritical
	default:
		return SeverityMedium
	}
}

// getHandler gets an error handler by code
func (h *Handler) getHandler(code ErrorCode) ErrorHandler {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return h.errorHandlers[code]
}

// getErrorWorkflow gets an error workflow by code
func (h *Handler) getErrorWorkflow(code ErrorCode) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return h.errorWorkflows[code]
}

// logError logs an error
func (h *Handler) logError(err ExecutionError) {
	h.logger.Error("Execution error occurred",
		"code", err.Code,
		"executionId", err.ExecutionID,
		"nodeId", err.NodeID,
		"severity", err.Severity,
		"retryable", err.Retryable,
		"message", err.Message,
	)
}

// triggerErrorWorkflow triggers an error workflow
func (h *Handler) triggerErrorWorkflow(ctx context.Context, workflowID string, err ExecutionError) {
	event := events.NewEventBuilder("error.workflow.trigger").
		WithPayload("workflowId", workflowID).
		WithPayload("error", err).
		Build()
	
	if publishErr := h.eventBus.Publish(ctx, event); publishErr != nil {
		h.logger.Error("Failed to trigger error workflow", "error", publishErr)
	}
}

// publishErrorEvent publishes an error event
func (h *Handler) publishErrorEvent(ctx context.Context, err ExecutionError) {
	event := events.NewEventBuilder("execution.error").
		WithAggregateID(err.ExecutionID).
		WithPayload("error", err).
		Build()
	
	if publishErr := h.eventBus.Publish(ctx, event); publishErr != nil {
		h.logger.Error("Failed to publish error event", "error", publishErr)
	}
}

// subscribeToEvents subscribes to relevant events
func (h *Handler) subscribeToEvents(ctx context.Context) error {
	return h.eventBus.Subscribe(events.ExecutionFailed, h.handleExecutionFailed)
}

// handleExecutionFailed handles execution failure events
func (h *Handler) handleExecutionFailed(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	errMsg, _ := event.Payload["error"].(string)
	
	err := ExecutionError{
		ExecutionID: executionID,
		Message:     errMsg,
		Cause:       fmt.Errorf(errMsg),
	}
	
	return h.HandleError(ctx, err)
}

// GetMetrics returns error handler metrics
func (h *Handler) GetMetrics() ErrorMetrics {
	return ErrorMetrics{
		TotalErrors:     h.totalErrors,
		HandledErrors:   h.handledErrors,
		UnhandledErrors: h.unhandledErrors,
	}
}

// ErrorMetrics contains error handler metrics
type ErrorMetrics struct {
	TotalErrors     int64 `json:"total_errors"`
	HandledErrors   int64 `json:"handled_errors"`
	UnhandledErrors int64 `json:"unhandled_errors"`
}

// Specific error handlers

// TimeoutErrorHandler handles timeout errors
type TimeoutErrorHandler struct {
	logger logger.Logger
}

func (h *TimeoutErrorHandler) Handle(ctx context.Context, err ExecutionError) error {
	h.logger.Info("Handling timeout error", "executionId", err.ExecutionID)
	// Implement timeout-specific handling
	return nil
}

func (h *TimeoutErrorHandler) CanHandle(err ExecutionError) bool {
	return err.Code == ErrorCodeTimeout
}

// RateLimitErrorHandler handles rate limit errors
type RateLimitErrorHandler struct {
	logger logger.Logger
}

func (h *RateLimitErrorHandler) Handle(ctx context.Context, err ExecutionError) error {
	h.logger.Info("Handling rate limit error", "executionId", err.ExecutionID)
	// Implement backoff and retry logic
	return nil
}

func (h *RateLimitErrorHandler) CanHandle(err ExecutionError) bool {
	return err.Code == ErrorCodeRateLimited
}

// ServiceErrorHandler handles service errors
type ServiceErrorHandler struct {
	logger logger.Logger
}

func (h *ServiceErrorHandler) Handle(ctx context.Context, err ExecutionError) error {
	h.logger.Info("Handling service error", "executionId", err.ExecutionID)
	// Implement circuit breaker logic
	return nil
}

func (h *ServiceErrorHandler) CanHandle(err ExecutionError) bool {
	return err.Code == ErrorCodeServiceUnavailable
}

// ScriptErrorHandler handles script errors
type ScriptErrorHandler struct {
	logger logger.Logger
}

func (h *ScriptErrorHandler) Handle(ctx context.Context, err ExecutionError) error {
	h.logger.Info("Handling script error", "executionId", err.ExecutionID)
	// Log detailed script error information
	return nil
}

func (h *ScriptErrorHandler) CanHandle(err ExecutionError) bool {
	return err.Code == ErrorCodeScriptError
}

// DatabaseErrorHandler handles database errors
type DatabaseErrorHandler struct {
	logger logger.Logger
}

func (h *DatabaseErrorHandler) Handle(ctx context.Context, err ExecutionError) error {
	h.logger.Info("Handling database error", "executionId", err.ExecutionID)
	// Implement database-specific error handling
	return nil
}

func (h *DatabaseErrorHandler) CanHandle(err ExecutionError) bool {
	return err.Code == ErrorCodeDatabaseError
}

// DefaultErrorClassifier is the default error classifier
type DefaultErrorClassifier struct{}

// NewDefaultErrorClassifier creates a new default error classifier
func NewDefaultErrorClassifier() *DefaultErrorClassifier {
	return &DefaultErrorClassifier{}
}

// IsRetryable determines if an error is retryable
func (c *DefaultErrorClassifier) IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	errMsg := strings.ToLower(err.Error())
	
	// Retryable error patterns
	retryablePatterns := []string{
		"timeout",
		"rate limit",
		"temporary",
		"unavailable",
		"connection refused",
		"network",
		"deadlock",
	}
	
	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	
	// Non-retryable error patterns
	nonRetryablePatterns := []string{
		"invalid",
		"permission denied",
		"not found",
		"unauthorized",
		"bad request",
		"syntax error",
	}
	
	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}
	
	// Default to retryable for unknown errors
	return true
}

// GetErrorType gets the error type
func (c *DefaultErrorClassifier) GetErrorType(err error) retry.ErrorType {
	if err == nil {
		return retry.ErrorTypeUnknown
	}
	
	errMsg := strings.ToLower(err.Error())
	
	switch {
	case strings.Contains(errMsg, "timeout"):
		return retry.ErrorTypeTimeout
	case strings.Contains(errMsg, "rate limit"):
		return retry.ErrorTypeRateLimit
	case strings.Contains(errMsg, "network"):
		return retry.ErrorTypeNetworkError
	case strings.Contains(errMsg, "service"):
		return retry.ErrorTypeServiceError
	case strings.Contains(errMsg, "temporary"):
		return retry.ErrorTypeTransient
	default:
		return retry.ErrorTypeUnknown
	}
}

// GetRetryStrategy gets the retry strategy for an error
func (c *DefaultErrorClassifier) GetRetryStrategy(err error) retry.Strategy {
	errorType := c.GetErrorType(err)
	
	switch errorType {
	case retry.ErrorTypeRateLimit:
		// Use exponential backoff for rate limits
		return retry.NewExponentialBackoffStrategy(5, 1*time.Second, 1*time.Minute, 2.0)
	case retry.ErrorTypeTimeout:
		// Use linear backoff for timeouts
		return retry.NewLinearBackoffStrategy(3, 5*time.Second)
	case retry.ErrorTypeTransient:
		// Use random jitter for transient errors
		return retry.NewRandomJitterStrategy(3, 1*time.Second, 10*time.Second)
	default:
		// Default to fixed delay
		return retry.NewFixedDelayStrategy(3, 2*time.Second)
	}
}
