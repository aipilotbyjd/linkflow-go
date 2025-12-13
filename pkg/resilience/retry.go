package resilience

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts       int
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
	Jitter            float64 // 0.0 to 1.0
	RetryableErrors   []error
	ShouldRetry       func(error) bool
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if cfg.ShouldRetry != nil && !cfg.ShouldRetry(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(cfg, attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return lastErr
}

// RetryWithResult executes a function with retry logic and returns a result
func RetryWithResult[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if cfg.ShouldRetry != nil && !cfg.ShouldRetry(err) {
			return zero, err
		}

		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(cfg, attempt)
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return zero, lastErr
}

func calculateDelay(cfg RetryConfig, attempt int) time.Duration {
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.BackoffMultiplier, float64(attempt))

	// Apply jitter
	if cfg.Jitter > 0 {
		jitterRange := delay * cfg.Jitter
		delay = delay - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return time.Duration(delay)
}

// IsRetryableHTTPStatus checks if an HTTP status code is retryable
func IsRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case 408, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}
