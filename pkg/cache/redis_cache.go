package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache interface using Redis
type RedisCache struct {
	client  *redis.Client
	options *Options
	codec   Codec
	metrics *CacheMetrics
	mu      sync.RWMutex
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(client *redis.Client, opts *Options) *RedisCache {
	if opts == nil {
		opts = DefaultOptions()
	}
	
	if opts.Codec == nil {
		opts.Codec = &JSONCodec{}
	}
	
	return &RedisCache{
		client:  client,
		options: opts,
		codec:   opts.Codec,
		metrics: &CacheMetrics{},
	}
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	start := time.Now()
	defer c.recordGetMetrics(start, key)
	
	key = c.buildKey(key)
	
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			c.incrementMisses()
			return ErrCacheMiss
		}
		return fmt.Errorf("redis get error: %w", err)
	}
	
	// Decompress if needed
	data = c.decompress(data)
	
	// Decode data
	if err := c.codec.Decode(data, dest); err != nil {
		return fmt.Errorf("decode error: %w", err)
	}
	
	c.incrementHits()
	
	// Update TTL if using sliding window
	if c.options.DefaultTTL > 0 {
		_ = c.client.Expire(ctx, key, c.options.DefaultTTL)
	}
	
	return nil
}

// Set stores a value in cache with TTL
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer c.recordSetMetrics(start, key)
	
	key = c.buildKey(key)
	
	// Encode value
	data, err := c.codec.Encode(value)
	if err != nil {
		return fmt.Errorf("encode error: %w", err)
	}
	
	// Compress if needed
	data = c.compress(data)
	
	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.options.DefaultTTL
	}
	
	// Set with retry
	err = c.retryOperation(func() error {
		return c.client.Set(ctx, key, data, ttl).Err()
	})
	
	if err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}
	
	c.incrementSets()
	return nil
}

// Delete removes a key from cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	key = c.buildKey(key)
	
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("redis delete error: %w", err)
	}
	
	c.incrementDeletes()
	return nil
}

// Exists checks if a key exists in cache
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	key = c.buildKey(key)
	
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists error: %w", err)
	}
	
	return exists > 0, nil
}

// Invalidate removes all keys matching a pattern
func (c *RedisCache) Invalidate(ctx context.Context, pattern string) error {
	pattern = c.buildKey(pattern)
	
	// Use SCAN to find keys matching pattern
	var cursor uint64
	var keys []string
	
	for {
		var err error
		var batch []string
		
		batch, cursor, err = c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("redis scan error: %w", err)
		}
		
		keys = append(keys, batch...)
		
		if cursor == 0 {
			break
		}
	}
	
	// Delete keys in batches
	if len(keys) > 0 {
		pipe := c.client.Pipeline()
		for _, key := range keys {
			pipe.Del(ctx, key)
		}
		
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("redis pipeline delete error: %w", err)
		}
	}
	
	return nil
}

// GetMulti retrieves multiple values from cache
func (c *RedisCache) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}
	
	// Build keys
	redisKeys := make([]string, len(keys))
	for i, key := range keys {
		redisKeys[i] = c.buildKey(key)
	}
	
	// Get values
	values, err := c.client.MGet(ctx, redisKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis mget error: %w", err)
	}
	
	// Decode values
	result := make(map[string]interface{})
	for i, value := range values {
		if value != nil {
			// Decompress and decode
			data := []byte(value.(string))
			data = c.decompress(data)
			
			var decoded interface{}
			if err := c.codec.Decode(data, &decoded); err == nil {
				result[keys[i]] = decoded
				c.incrementHits()
			}
		} else {
			c.incrementMisses()
		}
	}
	
	return result, nil
}

// SetMulti stores multiple values in cache
func (c *RedisCache) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}
	
	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.options.DefaultTTL
	}
	
	// Use pipeline for batch operations
	pipe := c.client.Pipeline()
	
	for key, value := range items {
		redisKey := c.buildKey(key)
		
		// Encode value
		data, err := c.codec.Encode(value)
		if err != nil {
			return fmt.Errorf("encode error for key %s: %w", key, err)
		}
		
		// Compress if needed
		data = c.compress(data)
		
		pipe.Set(ctx, redisKey, data, ttl)
		c.incrementSets()
	}
	
	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis pipeline set error: %w", err)
	}
	
	return nil
}

// Increment increments a counter in cache
func (c *RedisCache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	key = c.buildKey(key)
	
	result, err := c.client.IncrBy(ctx, key, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("redis increment error: %w", err)
	}
	
	return result, nil
}

// Decrement decrements a counter in cache
func (c *RedisCache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	key = c.buildKey(key)
	
	result, err := c.client.DecrBy(ctx, key, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("redis decrement error: %w", err)
	}
	
	return result, nil
}

// Expire sets a new TTL for a key
func (c *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	key = c.buildKey(key)
	
	ok, err := c.client.Expire(ctx, key, ttl).Result()
	if err != nil {
		return fmt.Errorf("redis expire error: %w", err)
	}
	
	if !ok {
		return ErrCacheMiss
	}
	
	return nil
}

// TTL returns the remaining TTL for a key
func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	key = c.buildKey(key)
	
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis ttl error: %w", err)
	}
	
	if ttl == -2 {
		return 0, ErrCacheMiss
	}
	
	return ttl, nil
}

// Flush removes all keys from cache
func (c *RedisCache) Flush(ctx context.Context) error {
	// If namespace is set, only flush keys with that namespace
	if c.options.Namespace != "" {
		return c.Invalidate(ctx, "*")
	}
	
	// Otherwise flush entire database (use with caution!)
	if err := c.client.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("redis flush error: %w", err)
	}
	
	return nil
}

// Close closes the cache connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Ping checks if cache is available
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// GetMetrics returns current cache metrics
func (c *RedisCache) GetMetrics() *CacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Get Redis info
	info := c.client.PoolStats()
	
	metrics := *c.metrics
	metrics.ConnectionPool = ConnectionPoolMetrics{
		ActiveConnections: int(info.TotalConns - info.IdleConns),
		IdleConnections:   int(info.IdleConns),
		TotalConnections:  int(info.TotalConns),
	}
	
	return &metrics
}

// Helper methods

func (c *RedisCache) buildKey(key string) string {
	if c.options.Namespace != "" {
		return fmt.Sprintf("%s:%s", c.options.Namespace, key)
	}
	return key
}

func (c *RedisCache) compress(data []byte) []byte {
	if len(data) < c.options.CompressionThreshold {
		return data
	}
	
	var buf bytes.Buffer
	buf.WriteByte(1) // Compression flag
	
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return data
	}
	gz.Close()
	
	return buf.Bytes()
}

func (c *RedisCache) decompress(data []byte) []byte {
	if len(data) == 0 || data[0] != 1 {
		return data
	}
	
	gz, err := gzip.NewReader(bytes.NewReader(data[1:]))
	if err != nil {
		return data
	}
	defer gz.Close()
	
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		return data
	}
	
	return decompressed
}

func (c *RedisCache) retryOperation(fn func() error) error {
	var err error
	for i := 0; i <= c.options.MaxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		
		if i < c.options.MaxRetries {
			time.Sleep(c.options.RetryDelay)
		}
	}
	return err
}

func (c *RedisCache) incrementHits() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.Hits++
}

func (c *RedisCache) incrementMisses() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.Misses++
}

func (c *RedisCache) incrementSets() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.Sets++
}

func (c *RedisCache) incrementDeletes() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.Deletes++
}

func (c *RedisCache) recordGetMetrics(start time.Time, key string) {
	if !c.options.EnableMetrics {
		return
	}
	
	duration := time.Since(start)
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Update average get time
	if c.metrics.AvgGetTime == 0 {
		c.metrics.AvgGetTime = duration
	} else {
		c.metrics.AvgGetTime = (c.metrics.AvgGetTime + duration) / 2
	}
}

func (c *RedisCache) recordSetMetrics(start time.Time, key string) {
	if !c.options.EnableMetrics {
		return
	}
	
	duration := time.Since(start)
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Update average set time
	if c.metrics.AvgSetTime == 0 {
		c.metrics.AvgSetTime = duration
	} else {
		c.metrics.AvgSetTime = (c.metrics.AvgSetTime + duration) / 2
	}
}

// WithNamespace creates a new cache instance with a specific namespace
func (c *RedisCache) WithNamespace(namespace string) Cache {
	newOptions := *c.options
	if newOptions.Namespace != "" {
		newOptions.Namespace = fmt.Sprintf("%s:%s", newOptions.Namespace, namespace)
	} else {
		newOptions.Namespace = namespace
	}
	
	return &RedisCache{
		client:  c.client,
		options: &newOptions,
		codec:   c.codec,
		metrics: c.metrics,
	}
}

// CacheAside implements cache-aside pattern
func (c *RedisCache) CacheAside(ctx context.Context, key string, dest interface{}, 
	loader func() (interface{}, error), ttl time.Duration) error {
	
	// Try to get from cache
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil // Cache hit
	}
	
	if err != ErrCacheMiss {
		return err // Actual error
	}
	
	// Cache miss - load from source
	value, err := loader()
	if err != nil {
		return fmt.Errorf("loader error: %w", err)
	}
	
	// Store in cache
	if err := c.Set(ctx, key, value, ttl); err != nil {
		// Log error but don't fail the operation
		// since we have the value from the loader
		_ = err
	}
	
	// Copy value to destination
	data, _ := c.codec.Encode(value)
	return c.codec.Decode(data, dest)
}
