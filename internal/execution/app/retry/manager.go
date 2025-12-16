package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/sony/gobreaker"
)

// Manager handles retry logic for executions
type Manager struct {
	mu              sync.RWMutex
	strategies      map[string]Strategy
	circuitBreakers map[string]*gobreaker.CircuitBreaker
	errorClassifier ErrorClassifier
	eventBus        events.EventBus
	logger          logger.Logger

	// Metrics
	totalRetries      int64
	successfulRetries int64
	failedRetries     int64

	// Control
	stopCh chan struct{}
}

// Strategy interface for retry strategies
type Strategy interface {
	ShouldRetry(err error, attempt int) bool
	NextDelay(attempt int) time.Duration
	MaxAttempts() int
	Name() string
}

// ErrorClassifier classifies errors for retry decisions
type ErrorClassifier interface {
	IsRetryable(err error) bool
	GetErrorType(err error) ErrorType
	GetRetryStrategy(err error) Strategy
}

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeTransient    ErrorType = "transient"
	ErrorTypeTimeout      ErrorType = "timeout"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeNetworkError ErrorType = "network_error"
	ErrorTypeServiceError ErrorType = "service_error"
	ErrorTypePermanent    ErrorType = "permanent"
	ErrorTypeUnknown      ErrorType = "unknown"
)

// NewManager creates a new retry manager
func NewManager(eventBus events.EventBus, logger logger.Logger) *Manager {
	manager := &Manager{
		strategies:      make(map[string]Strategy),
		circuitBreakers: make(map[string]*gobreaker.CircuitBreaker),
		errorClassifier: NewDefaultErrorClassifier(),
		eventBus:        eventBus,
		logger:          logger,
		stopCh:          make(chan struct{}),
	}

	// Register default strategies
	manager.RegisterStrategy("exponential", NewExponentialBackoffStrategy(3, 1*time.Second, 30*time.Second, 2.0))
	manager.RegisterStrategy("linear", NewLinearBackoffStrategy(3, 2*time.Second))
	manager.RegisterStrategy("fixed", NewFixedDelayStrategy(3, 5*time.Second))
	manager.RegisterStrategy("random", NewRandomJitterStrategy(3, 1*time.Second, 10*time.Second))

	return manager
}

// Start starts the retry manager
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting retry manager")

	// Subscribe to failure events
	if err := m.subscribeToEvents(ctx); err != nil {
		return err
	}

	return nil
}

// Stop stops the retry manager
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Stopping retry manager")
	close(m.stopCh)
	return nil
}

// RegisterStrategy registers a retry strategy
func (m *Manager) RegisterStrategy(name string, strategy Strategy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.strategies[name] = strategy
	m.logger.Info("Registered retry strategy", "name", name)
}

// GetStrategy gets a retry strategy by name
func (m *Manager) GetStrategy(name string) (Strategy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	strategy, exists := m.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy not found: %s", name)
	}

	return strategy, nil
}

// ShouldRetry determines if an operation should be retried
func (m *Manager) ShouldRetry(err error, attempt int, strategyName string) bool {
	// Check if error is retryable
	if !m.errorClassifier.IsRetryable(err) {
		m.logger.Debug("Error not retryable", "error", err)
		return false
	}

	// Get strategy
	strategy, err := m.GetStrategy(strategyName)
	if err != nil {
		// Use default strategy
		strategy = m.strategies["exponential"]
	}

	return strategy.ShouldRetry(err, attempt)
}

// GetNextDelay gets the delay before next retry
func (m *Manager) GetNextDelay(err error, attempt int, strategyName string) time.Duration {
	strategy, err := m.GetStrategy(strategyName)
	if err != nil {
		// Use default strategy
		strategy = m.strategies["exponential"]
	}

	return strategy.NextDelay(attempt)
}

// ExecuteWithRetry executes an operation with retry logic
func (m *Manager) ExecuteWithRetry(ctx context.Context, operation func() error, config RetryConfig) error {
	if config.Strategy == "" {
		config.Strategy = "exponential"
	}

	strategy, err := m.GetStrategy(config.Strategy)
	if err != nil {
		return err
	}

	// Get or create circuit breaker for this operation
	circuitBreaker := m.getOrCreateCircuitBreaker(config.OperationID)

	var lastErr error

	for attempt := 0; attempt < strategy.MaxAttempts(); attempt++ {
		// Check circuit breaker
		_, err := circuitBreaker.Execute(func() (interface{}, error) {
			// Execute operation
			lastErr = operation()
			return nil, lastErr
		})

		if err == nil {
			// Success
			m.successfulRetries++
			if attempt > 0 {
				m.logger.Info("Operation succeeded after retry",
					"operationId", config.OperationID,
					"attempt", attempt+1,
				)
			}
			return nil
		}

		// Check if we should retry
		if !m.ShouldRetry(err, attempt+1, config.Strategy) {
			break
		}

		// Calculate delay
		delay := strategy.NextDelay(attempt + 1)

		m.logger.Info("Retrying operation",
			"operationId", config.OperationID,
			"attempt", attempt+1,
			"delay", delay,
			"error", err,
		)

		m.totalRetries++

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}

		// Apply error workflow if configured
		if config.ErrorWorkflow != "" && attempt == strategy.MaxAttempts()-1 {
			m.triggerErrorWorkflow(ctx, config.ErrorWorkflow, err)
		}
	}

	m.failedRetries++
	return fmt.Errorf("operation failed after %d attempts: %w", strategy.MaxAttempts(), lastErr)
}

// getOrCreateCircuitBreaker gets or creates a circuit breaker
func (m *Manager) getOrCreateCircuitBreaker(operationID string) *gobreaker.CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cb, exists := m.circuitBreakers[operationID]; exists {
		return cb
	}

	// Create new circuit breaker
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        operationID,
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			m.logger.Info("Circuit breaker state changed",
				"name", name,
				"from", from,
				"to", to,
			)
		},
	})

	m.circuitBreakers[operationID] = cb
	return cb
}

// triggerErrorWorkflow triggers an error workflow
func (m *Manager) triggerErrorWorkflow(ctx context.Context, workflowID string, err error) {
	event := events.NewEventBuilder("error.workflow.trigger").
		WithPayload("workflowId", workflowID).
		WithPayload("error", err.Error()).
		Build()

	if err := m.eventBus.Publish(ctx, event); err != nil {
		m.logger.Error("Failed to trigger error workflow", "error", err)
	}
}

// subscribeToEvents subscribes to relevant events
func (m *Manager) subscribeToEvents(ctx context.Context) error {
	return m.eventBus.Subscribe("node.execution.failed", m.handleNodeExecutionFailed)
}

// handleNodeExecutionFailed handles node execution failure events
func (m *Manager) handleNodeExecutionFailed(ctx context.Context, event events.Event) error {
	nodeID, _ := event.Payload["nodeId"].(string)
	executionID, _ := event.Payload["executionId"].(string)

	m.logger.Info("Handling node execution failure",
		"nodeId", nodeID,
		"executionId", executionID,
	)

	// Determine retry strategy and trigger retry if needed
	// This is simplified - in production, would coordinate with orchestrator

	return nil
}

// GetMetrics returns retry metrics
func (m *Manager) GetMetrics() RetryMetrics {
	return RetryMetrics{
		TotalRetries:      m.totalRetries,
		SuccessfulRetries: m.successfulRetries,
		FailedRetries:     m.failedRetries,
	}
}

// RetryConfig contains configuration for retry
type RetryConfig struct {
	OperationID   string
	Strategy      string
	ErrorWorkflow string
}

// RetryMetrics contains retry metrics
type RetryMetrics struct {
	TotalRetries      int64 `json:"total_retries"`
	SuccessfulRetries int64 `json:"successful_retries"`
	FailedRetries     int64 `json:"failed_retries"`
}

// ExponentialBackoffStrategy implements exponential backoff
type ExponentialBackoffStrategy struct {
	maxAttempts   int
	initialDelay  time.Duration
	maxDelay      time.Duration
	backoffFactor float64
}

// NewExponentialBackoffStrategy creates a new exponential backoff strategy
func NewExponentialBackoffStrategy(maxAttempts int, initialDelay, maxDelay time.Duration, backoffFactor float64) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		maxAttempts:   maxAttempts,
		initialDelay:  initialDelay,
		maxDelay:      maxDelay,
		backoffFactor: backoffFactor,
	}
}

func (s *ExponentialBackoffStrategy) ShouldRetry(err error, attempt int) bool {
	return attempt <= s.maxAttempts
}

func (s *ExponentialBackoffStrategy) NextDelay(attempt int) time.Duration {
	delay := float64(s.initialDelay) * math.Pow(s.backoffFactor, float64(attempt-1))
	if delay > float64(s.maxDelay) {
		return s.maxDelay
	}
	return time.Duration(delay)
}

func (s *ExponentialBackoffStrategy) MaxAttempts() int {
	return s.maxAttempts
}

func (s *ExponentialBackoffStrategy) Name() string {
	return "exponential"
}

// LinearBackoffStrategy implements linear backoff
type LinearBackoffStrategy struct {
	maxAttempts    int
	delayIncrement time.Duration
}

// NewLinearBackoffStrategy creates a new linear backoff strategy
func NewLinearBackoffStrategy(maxAttempts int, delayIncrement time.Duration) *LinearBackoffStrategy {
	return &LinearBackoffStrategy{
		maxAttempts:    maxAttempts,
		delayIncrement: delayIncrement,
	}
}

func (s *LinearBackoffStrategy) ShouldRetry(err error, attempt int) bool {
	return attempt <= s.maxAttempts
}

func (s *LinearBackoffStrategy) NextDelay(attempt int) time.Duration {
	return s.delayIncrement * time.Duration(attempt)
}

func (s *LinearBackoffStrategy) MaxAttempts() int {
	return s.maxAttempts
}

func (s *LinearBackoffStrategy) Name() string {
	return "linear"
}

// FixedDelayStrategy implements fixed delay retry
type FixedDelayStrategy struct {
	maxAttempts int
	delay       time.Duration
}

// NewFixedDelayStrategy creates a new fixed delay strategy
func NewFixedDelayStrategy(maxAttempts int, delay time.Duration) *FixedDelayStrategy {
	return &FixedDelayStrategy{
		maxAttempts: maxAttempts,
		delay:       delay,
	}
}

func (s *FixedDelayStrategy) ShouldRetry(err error, attempt int) bool {
	return attempt <= s.maxAttempts
}

func (s *FixedDelayStrategy) NextDelay(attempt int) time.Duration {
	return s.delay
}

func (s *FixedDelayStrategy) MaxAttempts() int {
	return s.maxAttempts
}

func (s *FixedDelayStrategy) Name() string {
	return "fixed"
}

// RandomJitterStrategy implements random jitter retry
type RandomJitterStrategy struct {
	maxAttempts int
	minDelay    time.Duration
	maxDelay    time.Duration
}

// NewRandomJitterStrategy creates a new random jitter strategy
func NewRandomJitterStrategy(maxAttempts int, minDelay, maxDelay time.Duration) *RandomJitterStrategy {
	return &RandomJitterStrategy{
		maxAttempts: maxAttempts,
		minDelay:    minDelay,
		maxDelay:    maxDelay,
	}
}

func (s *RandomJitterStrategy) ShouldRetry(err error, attempt int) bool {
	return attempt <= s.maxAttempts
}

func (s *RandomJitterStrategy) NextDelay(attempt int) time.Duration {
	jitter := rand.Int63n(int64(s.maxDelay - s.minDelay))
	return s.minDelay + time.Duration(jitter)
}

func (s *RandomJitterStrategy) MaxAttempts() int {
	return s.maxAttempts
}

func (s *RandomJitterStrategy) Name() string {
	return "random"
}

// DefaultErrorClassifier is the default implementation of ErrorClassifier
type DefaultErrorClassifier struct {
	retryableErrorPatterns []string
}

// NewDefaultErrorClassifier creates a new default error classifier
func NewDefaultErrorClassifier() *DefaultErrorClassifier {
	return &DefaultErrorClassifier{
		retryableErrorPatterns: []string{
			"timeout",
			"connection refused",
			"connection reset",
			"temporary failure",
			"rate limit",
			"429",
			"503",
			"504",
			"network",
			"EOF",
		},
	}
}

// IsRetryable determines if an error is retryable
func (c *DefaultErrorClassifier) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, pattern := range c.retryableErrorPatterns {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// GetErrorType classifies the error type
func (c *DefaultErrorClassifier) GetErrorType(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "timeout") {
		return ErrorTypeTimeout
	}
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") {
		return ErrorTypeRateLimit
	}
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") {
		return ErrorTypeNetworkError
	}
	if strings.Contains(errStr, "503") || strings.Contains(errStr, "504") {
		return ErrorTypeServiceError
	}
	if strings.Contains(errStr, "temporary") {
		return ErrorTypeTransient
	}

	return ErrorTypePermanent
}

// GetRetryStrategy returns the appropriate retry strategy for an error
func (c *DefaultErrorClassifier) GetRetryStrategy(err error) Strategy {
	errorType := c.GetErrorType(err)

	switch errorType {
	case ErrorTypeRateLimit:
		return NewExponentialBackoffStrategy(5, 5*time.Second, 60*time.Second, 2.0)
	case ErrorTypeTimeout, ErrorTypeNetworkError:
		return NewExponentialBackoffStrategy(3, 1*time.Second, 30*time.Second, 2.0)
	case ErrorTypeServiceError:
		return NewFixedDelayStrategy(3, 10*time.Second)
	default:
		return NewExponentialBackoffStrategy(3, 1*time.Second, 30*time.Second, 2.0)
	}
}
