package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	// ErrCacheMiss is returned when a key is not found in cache
	ErrCacheMiss = errors.New("cache miss")
	// ErrCacheInvalidated is returned when a cache entry has been invalidated
	ErrCacheInvalidated = errors.New("cache invalidated")
)

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string, dest interface{}) error

	// Set stores a value in cache with TTL
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a key from cache
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in cache
	Exists(ctx context.Context, key string) (bool, error)

	// Invalidate removes all keys matching a pattern
	Invalidate(ctx context.Context, pattern string) error

	// GetMulti retrieves multiple values from cache
	GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)

	// SetMulti stores multiple values in cache
	SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// Increment increments a counter in cache
	Increment(ctx context.Context, key string, delta int64) (int64, error)

	// Decrement decrements a counter in cache
	Decrement(ctx context.Context, key string, delta int64) (int64, error)

	// Expire sets a new TTL for a key
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// TTL returns the remaining TTL for a key
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Flush removes all keys from cache
	Flush(ctx context.Context) error

	// Close closes the cache connection
	Close() error

	// Ping checks if cache is available
	Ping(ctx context.Context) error
}

// Codec defines the interface for encoding/decoding cache values
type Codec interface {
	Encode(value interface{}) ([]byte, error)
	Decode(data []byte, dest interface{}) error
}

// JSONCodec implements Codec using JSON encoding
type JSONCodec struct{}

// Encode encodes a value to JSON bytes
func (c *JSONCodec) Encode(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

// Decode decodes JSON bytes to a value
func (c *JSONCodec) Decode(data []byte, dest interface{}) error {
	return json.Unmarshal(data, dest)
}

// Options represents cache configuration options
type Options struct {
	// DefaultTTL is the default TTL for cache entries
	DefaultTTL time.Duration

	// MaxRetries is the maximum number of retries for cache operations
	MaxRetries int

	// RetryDelay is the delay between retries
	RetryDelay time.Duration

	// Namespace is a prefix for all cache keys
	Namespace string

	// Codec is the encoder/decoder for cache values
	Codec Codec

	// EnableMetrics enables metrics collection
	EnableMetrics bool

	// CompressionThreshold is the minimum size in bytes to enable compression
	CompressionThreshold int
}

// DefaultOptions returns default cache options
func DefaultOptions() *Options {
	return &Options{
		DefaultTTL:           5 * time.Minute,
		MaxRetries:           3,
		RetryDelay:           100 * time.Millisecond,
		Namespace:            "",
		Codec:                &JSONCodec{},
		EnableMetrics:        true,
		CompressionThreshold: 1024, // 1KB
	}
}

// CacheMetrics represents cache performance metrics
type CacheMetrics struct {
	Hits           int64
	Misses         int64
	Sets           int64
	Deletes        int64
	Evictions      int64
	TotalKeys      int64
	TotalSize      int64
	AvgGetTime     time.Duration
	AvgSetTime     time.Duration
	ConnectionPool ConnectionPoolMetrics
}

// ConnectionPoolMetrics represents connection pool metrics
type ConnectionPoolMetrics struct {
	ActiveConnections int
	IdleConnections   int
	TotalConnections  int
	WaitingRequests   int
	TimeoutErrors     int64
	ConnectionErrors  int64
}

// CacheEntry represents a cache entry with metadata
type CacheEntry struct {
	Key          string
	Value        interface{}
	TTL          time.Duration
	CreatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int64
	Size         int64
}

// CacheKeyBuilder helps build cache keys with consistent formatting
type CacheKeyBuilder struct {
	namespace string
	separator string
}

// NewCacheKeyBuilder creates a new cache key builder
func NewCacheKeyBuilder(namespace string) *CacheKeyBuilder {
	return &CacheKeyBuilder{
		namespace: namespace,
		separator: ":",
	}
}

// Build builds a cache key from parts
func (b *CacheKeyBuilder) Build(parts ...string) string {
	if b.namespace != "" {
		parts = append([]string{b.namespace}, parts...)
	}

	key := ""
	for i, part := range parts {
		if i > 0 {
			key += b.separator
		}
		key += part
	}

	return key
}

// Pattern builds a pattern for cache invalidation
func (b *CacheKeyBuilder) Pattern(parts ...string) string {
	key := b.Build(parts...)
	return key + "*"
}

// CacheStrategy defines different caching strategies
type CacheStrategy string

const (
	// WriteThrough writes to both cache and database
	WriteThrough CacheStrategy = "write_through"

	// WriteBehind writes to cache first, database asynchronously
	WriteBehind CacheStrategy = "write_behind"

	// CacheAside reads from cache, loads from database on miss
	CacheAside CacheStrategy = "cache_aside"

	// ReadThrough reads through cache, auto-loads on miss
	ReadThrough CacheStrategy = "read_through"

	// Refresh periodically refreshes cache from database
	Refresh CacheStrategy = "refresh"
)

// TTLStrategy defines how TTL is managed
type TTLStrategy string

const (
	// FixedTTL uses a fixed TTL for all entries
	FixedTTL TTLStrategy = "fixed"

	// SlidingTTL extends TTL on each access
	SlidingTTL TTLStrategy = "sliding"

	// AdaptiveTTL adjusts TTL based on access patterns
	AdaptiveTTL TTLStrategy = "adaptive"
)

// CacheWarmer interface for warming up cache
type CacheWarmer interface {
	// WarmUp pre-loads frequently accessed data into cache
	WarmUp(ctx context.Context) error

	// GetWarmUpKeys returns keys that should be warmed up
	GetWarmUpKeys() []string

	// IsWarm checks if cache is warmed up
	IsWarm() bool
}
