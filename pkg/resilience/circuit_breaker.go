package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests")
)

// CircuitBreaker wraps sony/gobreaker with additional functionality
type CircuitBreaker struct {
	cb   *gobreaker.CircuitBreaker
	name string
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Name          string
	MaxRequests   uint32        // Max requests in half-open state
	Interval      time.Duration // Cyclic period for clearing counts
	Timeout       time.Duration // Period of open state before half-open
	FailureRatio  float64       // Failure ratio to trip the breaker
	MinRequests   uint32        // Minimum requests before evaluating
	OnStateChange func(name string, from, to gobreaker.State)
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:         name,
		MaxRequests:  3,
		Interval:     30 * time.Second,
		Timeout:      30 * time.Second,
		FailureRatio: 0.5,
		MinRequests:  5,
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < cfg.MinRequests {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= cfg.FailureRatio
		},
		OnStateChange: cfg.OnStateChange,
	}

	return &CircuitBreaker{
		cb:   gobreaker.NewCircuitBreaker(settings),
		name: cfg.Name,
	}
}

// Execute runs the given function with circuit breaker protection
func (c *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	return c.cb.Execute(fn)
}

// ExecuteWithContext runs the given function with context and circuit breaker protection
func (c *CircuitBreaker) ExecuteWithContext(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	result, err := c.cb.Execute(func() (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return fn(ctx)
		}
	})
	return result, err
}

// State returns the current state of the circuit breaker
func (c *CircuitBreaker) State() gobreaker.State {
	return c.cb.State()
}

// Name returns the name of the circuit breaker
func (c *CircuitBreaker) Name() string {
	return c.name
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
	config   CircuitBreakerConfig
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry(defaultConfig CircuitBreakerConfig) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   defaultConfig,
	}
}

// Get returns a circuit breaker by name, creating one if it doesn't exist
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, exists := r.breakers[name]
	r.mu.RUnlock()

	if exists {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists = r.breakers[name]; exists {
		return cb
	}

	cfg := r.config
	cfg.Name = name
	cb = NewCircuitBreaker(cfg)
	r.breakers[name] = cb

	return cb
}

// States returns the states of all circuit breakers
func (r *CircuitBreakerRegistry) States() map[string]gobreaker.State {
	r.mu.RLock()
	defer r.mu.RUnlock()

	states := make(map[string]gobreaker.State)
	for name, cb := range r.breakers {
		states[name] = cb.State()
	}
	return states
}
