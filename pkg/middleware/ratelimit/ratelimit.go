package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter interface for different rate limiting strategies
type RateLimiter interface {
	Allow(key string) bool
	Reset(key string)
}

// InMemoryRateLimiter implements rate limiting using in-memory storage
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

func (r *InMemoryRateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	now := time.Now()
	
	info, exists := r.attempts[key]
	if !exists {
		r.attempts[key] = &attemptInfo{
			count:     1,
			firstTime: now,
		}
		return true
	}
	
	// Check if blocked
	if info.blocked {
		// Check if block period has expired (15 minutes)
		if now.Sub(info.blockTime) > 15*time.Minute {
			// Reset the block
			info.blocked = false
			info.count = 1
			info.firstTime = now
			return true
		}
		return false
	}
	
	// Check if window has expired
	if now.Sub(info.firstTime) > r.window {
		// Reset the counter
		info.count = 1
		info.firstTime = now
		return true
	}
	
	// Increment counter
	info.count++
	
	// Check if exceeded max retries
	if info.count > r.maxRetries {
		info.blocked = true
		info.blockTime = now
		return false
	}
	
	return true
}

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

// RedisRateLimiter implements rate limiting using Redis
type RedisRateLimiter struct {
	client     *redis.Client
	maxRetries int
	window     time.Duration
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(client *redis.Client, maxRetries int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:     client,
		maxRetries: maxRetries,
		window:     window,
	}
}

func (r *RedisRateLimiter) Allow(key string) bool {
	ctx := context.Background()
	
	// Check if blocked
	blockedKey := fmt.Sprintf("blocked:%s", key)
	blocked, _ := r.client.Exists(ctx, blockedKey).Result()
	if blocked > 0 {
		return false
	}
	
	// Increment counter
	attemptsKey := fmt.Sprintf("attempts:%s", key)
	count, _ := r.client.Incr(ctx, attemptsKey).Result()
	
	// Set expiry on first attempt
	if count == 1 {
		r.client.Expire(ctx, attemptsKey, r.window)
	}
	
	// Check if exceeded max retries
	if int(count) > r.maxRetries {
		// Block for 15 minutes
		r.client.Set(ctx, blockedKey, "1", 15*time.Minute)
		r.client.Del(ctx, attemptsKey)
		return false
	}
	
	return true
}

func (r *RedisRateLimiter) Reset(key string) {
	ctx := context.Background()
	attemptsKey := fmt.Sprintf("attempts:%s", key)
	blockedKey := fmt.Sprintf("blocked:%s", key)
	r.client.Del(ctx, attemptsKey, blockedKey)
}

// LoginRateLimitMiddleware creates a middleware for rate limiting login attempts
func LoginRateLimitMiddleware(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use IP address as the key for rate limiting
		key := c.ClientIP()
		
		// For login endpoints, also consider email if provided
		if c.Request.URL.Path == "/auth/login" && c.Request.Method == "POST" {
			var req struct {
				Email string `json:"email"`
			}
			if err := c.ShouldBindJSON(&req); err == nil && req.Email != "" {
				// Use combination of IP and email
				key = fmt.Sprintf("%s:%s", c.ClientIP(), req.Email)
			}
			// Reset request body for next handler
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)
		}
		
		if !limiter.Allow(key) {
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

// GeneralRateLimitMiddleware creates a general rate limiting middleware
func GeneralRateLimitMiddleware(requests int, window time.Duration, redis *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		key := fmt.Sprintf("rate_limit:%s:%s", c.ClientIP(), c.Request.URL.Path)
		
		// If Redis is available, use it
		if redis != nil {
			count, _ := redis.Incr(ctx, key).Result()
			if count == 1 {
				redis.Expire(ctx, key, window)
			}
			
			if int(count) > requests {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded",
				})
				c.Abort()
				return
			}
		}
		
		c.Next()
	}
}
