package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// RateLimiter interface for different rate limiting strategies
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	Limit() rate.Limit
	Burst() int
}

// TokenBucketLimiter implements token bucket algorithm
type TokenBucketLimiter struct {
	limiter *rate.Limiter
}

func NewTokenBucketLimiter(rps int, burst int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

func (l *TokenBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.limiter.Allow(), nil
}

func (l *TokenBucketLimiter) Limit() rate.Limit {
	return l.limiter.Limit()
}

func (l *TokenBucketLimiter) Burst() int {
	return l.limiter.Burst()
}

// RedisRateLimiter implements distributed rate limiting using Redis
type RedisRateLimiter struct {
	redis  *redis.Client
	limit  int
	window time.Duration
}

func NewRedisRateLimiter(client *redis.Client, limit int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		redis:  client,
		limit:  limit,
		window: window,
	}
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - int64(r.window.Seconds())

	pipe := r.redis.Pipeline()

	// Remove old entries
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))

	// Count current entries
	countCmd := pipe.ZCard(ctx, key)

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	count := countCmd.Val()

	// Check if limit exceeded
	if count >= int64(r.limit) {
		return false, nil
	}

	// Add new entry
	member := fmt.Sprintf("%d:%s", now, key)
	if err := r.redis.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: member,
	}).Err(); err != nil {
		return false, fmt.Errorf("failed to add entry: %w", err)
	}

	// Set expiry
	r.redis.Expire(ctx, key, r.window)

	return true, nil
}

func (r *RedisRateLimiter) Limit() rate.Limit {
	return rate.Limit(float64(r.limit) / r.window.Seconds())
}

func (r *RedisRateLimiter) Burst() int {
	return r.limit
}

// SlidingWindowLimiter implements sliding window algorithm
type SlidingWindowLimiter struct {
	redis  *redis.Client
	limit  int
	window time.Duration
}

func NewSlidingWindowLimiter(client *redis.Client, limit int, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		redis:  client,
		limit:  limit,
		window: window,
	}
}

func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now()
	currentWindow := now.Unix() / int64(s.window.Seconds())
	previousWindow := currentWindow - 1

	currentKey := fmt.Sprintf("%s:%d", key, currentWindow)
	previousKey := fmt.Sprintf("%s:%d", key, previousWindow)

	// Get counts from both windows
	pipe := s.redis.Pipeline()
	currentCountCmd := pipe.Get(ctx, currentKey)
	previousCountCmd := pipe.Get(ctx, previousKey)
	pipe.Exec(ctx)

	currentCount, _ := strconv.Atoi(currentCountCmd.Val())
	previousCount, _ := strconv.Atoi(previousCountCmd.Val())

	// Calculate weighted count
	windowProgress := float64(now.Unix()%int64(s.window.Seconds())) / s.window.Seconds()
	weightedCount := float64(previousCount)*(1-windowProgress) + float64(currentCount)

	if weightedCount >= float64(s.limit) {
		return false, nil
	}

	// Increment current window counter
	pipe = s.redis.Pipeline()
	pipe.Incr(ctx, currentKey)
	pipe.Expire(ctx, currentKey, s.window*2)
	pipe.Exec(ctx)

	return true, nil
}

func (s *SlidingWindowLimiter) Limit() rate.Limit {
	return rate.Limit(float64(s.limit) / s.window.Seconds())
}

func (s *SlidingWindowLimiter) Burst() int {
	return s.limit
}

// Middleware creates a Gin middleware for rate limiting
func Middleware(limiter RateLimiter, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFunc(c)
		if key == "" {
			key = c.ClientIP()
		}

		allowed, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Rate limiting error",
			})
			c.Abort()
			return
		}

		if !allowed {
			c.Header("X-RateLimit-Limit", strconv.Itoa(limiter.Burst()))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// IPKeyFunc returns client IP as rate limit key
func IPKeyFunc(c *gin.Context) string {
	return c.ClientIP()
}

// UserKeyFunc returns user ID as rate limit key
func UserKeyFunc(c *gin.Context) string {
	userID, exists := c.Get("userId")
	if !exists {
		return c.ClientIP()
	}
	return fmt.Sprintf("user:%v", userID)
}

// APIKeyFunc returns API key as rate limit key
func APIKeyFunc(c *gin.Context) string {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		return c.ClientIP()
	}
	return fmt.Sprintf("api:%s", apiKey)
}

// TieredRateLimiter implements tiered rate limiting based on user plan
type TieredRateLimiter struct {
	redis *redis.Client
	tiers map[string]RateLimitTier
}

type RateLimitTier struct {
	Limit  int
	Window time.Duration
	Burst  int
}

func NewTieredRateLimiter(client *redis.Client) *TieredRateLimiter {
	return &TieredRateLimiter{
		redis: client,
		tiers: map[string]RateLimitTier{
			"free": {
				Limit:  100,
				Window: time.Hour,
				Burst:  10,
			},
			"basic": {
				Limit:  1000,
				Window: time.Hour,
				Burst:  50,
			},
			"premium": {
				Limit:  10000,
				Window: time.Hour,
				Burst:  200,
			},
			"enterprise": {
				Limit:  100000,
				Window: time.Hour,
				Burst:  1000,
			},
		},
	}
}

func (t *TieredRateLimiter) GetLimiter(tier string) RateLimiter {
	config, exists := t.tiers[tier]
	if !exists {
		config = t.tiers["free"]
	}

	return NewRedisRateLimiter(t.redis, config.Limit, config.Window)
}

// DynamicRateLimiter adjusts rate limits based on system load
type DynamicRateLimiter struct {
	base        RateLimiter
	loadFunc    func() float64
	adjustRatio float64
}

func NewDynamicRateLimiter(base RateLimiter, loadFunc func() float64) *DynamicRateLimiter {
	return &DynamicRateLimiter{
		base:        base,
		loadFunc:    loadFunc,
		adjustRatio: 1.0,
	}
}

func (d *DynamicRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	// Adjust ratio based on load
	load := d.loadFunc()
	if load > 0.8 {
		d.adjustRatio = 0.5 // Reduce limit by 50%
	} else if load > 0.6 {
		d.adjustRatio = 0.75 // Reduce limit by 25%
	} else {
		d.adjustRatio = 1.0 // Full limit
	}

	// Apply adjusted limit
	if d.adjustRatio < 1.0 {
		// Use random sampling to achieve fractional rate limiting
		if time.Now().UnixNano()%100 > int64(d.adjustRatio*100) {
			return false, nil
		}
	}

	return d.base.Allow(ctx, key)
}

func (d *DynamicRateLimiter) Limit() rate.Limit {
	return rate.Limit(float64(d.base.Limit()) * d.adjustRatio)
}

func (d *DynamicRateLimiter) Burst() int {
	return int(float64(d.base.Burst()) * d.adjustRatio)
}

// InMemoryRateLimiter implements rate limiting using in-memory storage
// Useful for login attempt limiting without Redis dependency
type InMemoryRateLimiter struct {
	mu         sync.Mutex
	attempts   map[string]*attemptInfo
	maxRetries int
	window     time.Duration
}

type attemptInfo struct {
	count     int
	firstTime time.Time
	blocked   bool
	blockTime time.Time
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
func NewInMemoryRateLimiter(maxRetries int, window time.Duration) *InMemoryRateLimiter {
	limiter := &InMemoryRateLimiter{
		attempts:   make(map[string]*attemptInfo),
		maxRetries: maxRetries,
		window:     window,
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

func (r *InMemoryRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	info, exists := r.attempts[key]
	if !exists {
		r.attempts[key] = &attemptInfo{
			count:     1,
			firstTime: now,
		}
		return true, nil
	}

	// Check if blocked
	if info.blocked {
		// Check if block period has expired (15 minutes)
		if now.Sub(info.blockTime) > 15*time.Minute {
			// Reset the block
			info.blocked = false
			info.count = 1
			info.firstTime = now
			return true, nil
		}
		return false, nil
	}

	// Check if window has expired
	if now.Sub(info.firstTime) > r.window {
		// Reset the counter
		info.count = 1
		info.firstTime = now
		return true, nil
	}

	// Increment counter
	info.count++

	// Check if exceeded max retries
	if info.count > r.maxRetries {
		info.blocked = true
		info.blockTime = now
		return false, nil
	}

	return true, nil
}

func (r *InMemoryRateLimiter) Limit() rate.Limit {
	return rate.Limit(float64(r.maxRetries) / r.window.Seconds())
}

func (r *InMemoryRateLimiter) Burst() int {
	return r.maxRetries
}

// Reset clears rate limit state for a key (e.g., after successful login)
func (r *InMemoryRateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, key)
}

func (r *InMemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for key, info := range r.attempts {
			// Remove entries older than 1 hour
			if now.Sub(info.firstTime) > time.Hour {
				delete(r.attempts, key)
			}
		}
		r.mu.Unlock()
	}
}

// LoginRateLimitMiddleware creates a middleware for rate limiting login attempts
func LoginRateLimitMiddleware(limiter *InMemoryRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use IP address as the key for rate limiting
		key := c.ClientIP()

		allowed, _ := limiter.Allow(c.Request.Context(), key)
		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many failed login attempts. Please try again later.",
			})
			c.Abort()
			return
		}

		// Continue to next handler
		c.Next()

		// If login was successful (status 200), reset the rate limiter
		if c.Writer.Status() == http.StatusOK {
			limiter.Reset(key)
		}
	}
}
