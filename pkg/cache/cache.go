package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// CacheConfig holds cache configuration
type CacheConfig struct {
	RedisURL         string        `json:"redis_url" validate:"required"`
	DefaultTTL       time.Duration `json:"default_ttl" validate:"min=1s"`
	MaxRetries       int           `json:"max_retries" validate:"min=1,max=10"`
	RetryDelay       time.Duration `json:"retry_delay" validate:"min=100ms"`
	PoolSize         int           `json:"pool_size" validate:"min=1,max=100"`
	EnableMetrics    bool          `json:"enable_metrics"`
	Namespace        string        `json:"namespace"`
	CompressionLevel int           `json:"compression_level" validate:"min=0,max=9"`
	WarmupKeys       []string      `json:"warmup_keys"`
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		RedisURL:         "redis://localhost:6379",
		DefaultTTL:       1 * time.Hour,
		MaxRetries:       3,
		RetryDelay:       100 * time.Millisecond,
		PoolSize:         10,
		EnableMetrics:    true,
		Namespace:        "docworker",
		CompressionLevel: 6,
		WarmupKeys:       []string{},
	}
}

// CacheEntry represents a cached value with metadata
type CacheEntry struct {
	Value       interface{} `json:"value"`
	TTL         int64       `json:"ttl"`
	CreatedAt   time.Time   `json:"created_at"`
	AccessCount int64       `json:"access_count"`
	Tags        []string    `json:"tags,omitempty"`
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	Hits           int64         `json:"hits"`
	Misses         int64         `json:"misses"`
	Sets           int64         `json:"sets"`
	Deletes        int64         `json:"deletes"`
	Evictions      int64         `json:"evictions"`
	Size           int64         `json:"size"`
	Memory         int64         `json:"memory"`
	AverageLatency time.Duration `json:"average_latency"`
	HitRatio       float64       `json:"hit_ratio"`
	LastUpdated    time.Time     `json:"last_updated"`
	mu             sync.RWMutex
}

// CacheMetrics interface for metrics recording
type CacheMetrics interface {
	RecordCacheOperation(operation, result string, latency time.Duration, size int64)
	RecordCacheSize(size int64)
	RecordCacheMemory(memory int64)
}

// Cache provides Redis-based caching with advanced features
type Cache struct {
	client   *redis.Client
	config   *CacheConfig
	stats    *CacheStats
	metrics  CacheMetrics
	logger   zerolog.Logger
	mu       sync.RWMutex
	patterns map[string]time.Duration // TTL patterns for different key types
}

// NewCache creates a new cache instance
func NewCache(config *CacheConfig, logger zerolog.Logger, metrics CacheMetrics) (*Cache, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	// Parse Redis URL
	opt, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	opt.PoolSize = config.PoolSize
	opt.MaxRetries = config.MaxRetries
	opt.MinRetryBackoff = config.RetryDelay

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	cache := &Cache{
		client:   client,
		config:   config,
		stats:    &CacheStats{LastUpdated: time.Now()},
		metrics:  metrics,
		logger:   logger.With().Str("component", "cache").Logger(),
		patterns: make(map[string]time.Duration),
	}

	// Set up default TTL patterns
	cache.SetTTLPattern("user:*", 30*time.Minute)
	cache.SetTTLPattern("session:*", 15*time.Minute)
	cache.SetTTLPattern("document:*", 2*time.Hour)
	cache.SetTTLPattern("temp:*", 5*time.Minute)

	cache.logger.Info().
		Str("redis_url", config.RedisURL).
		Dur("default_ttl", config.DefaultTTL).
		Int("pool_size", config.PoolSize).
		Msg("Cache initialized")

	// Perform warmup if configured
	if len(config.WarmupKeys) > 0 {
		go cache.warmup(context.Background())
	}

	return cache, nil
}

// SetTTLPattern sets TTL for keys matching a pattern
func (c *Cache) SetTTLPattern(pattern string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.patterns[pattern] = ttl
}

// getTTLForKey returns TTL for a specific key based on patterns
func (c *Cache) getTTLForKey(key string) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for pattern, ttl := range c.patterns {
		// Simple pattern matching (supports * wildcard)
		if matchPattern(pattern, key) {
			return ttl
		}
	}
	return c.config.DefaultTTL
}

// matchPattern performs simple wildcard pattern matching
func matchPattern(pattern, key string) bool {
	if pattern == "*" {
		return true
	}

	// Handle patterns with * at the end
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}

	return pattern == key
}

// buildKey creates a namespaced key
func (c *Cache) buildKey(key string) string {
	if c.config.Namespace == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", c.config.Namespace, key)
}

// Set stores a value in cache with optional TTL
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		c.updateStats(func(s *CacheStats) {
			s.Sets++
			s.AverageLatency = (s.AverageLatency + latency) / 2
			s.LastUpdated = time.Now()
		})

		if c.metrics != nil {
			size := estimateSize(value)
			c.metrics.RecordCacheOperation("set", "success", latency, size)
		}
	}()

	finalKey := c.buildKey(key)

	// Determine TTL
	var finalTTL time.Duration
	if len(ttl) > 0 && ttl[0] > 0 {
		finalTTL = ttl[0]
	} else {
		finalTTL = c.getTTLForKey(key)
	}

	// Create cache entry
	entry := CacheEntry{
		Value:       value,
		TTL:         int64(finalTTL.Seconds()),
		CreatedAt:   time.Now(),
		AccessCount: 0,
	}

	// Serialize entry
	data, err := json.Marshal(entry)
	if err != nil {
		c.logger.Error().Err(err).Str("key", key).Msg("Failed to marshal cache entry")
		return fmt.Errorf("marshal error: %w", err)
	}

	// Store in Redis
	if err := c.client.Set(ctx, finalKey, data, finalTTL).Err(); err != nil {
		c.logger.Error().Err(err).Str("key", key).Msg("Failed to set cache value")
		return fmt.Errorf("redis set error: %w", err)
	}

	c.logger.Debug().
		Str("key", key).
		Dur("ttl", finalTTL).
		Int("size", len(data)).
		Msg("Cache value set")

	return nil
}

// Get retrieves a value from cache
func (c *Cache) Get(ctx context.Context, key string) (interface{}, error) {
	start := time.Now()
	var hit bool

	defer func() {
		latency := time.Since(start)
		c.updateStats(func(s *CacheStats) {
			if hit {
				s.Hits++
			} else {
				s.Misses++
			}
			s.AverageLatency = (s.AverageLatency + latency) / 2
			s.LastUpdated = time.Now()
		})

		if c.metrics != nil {
			result := "miss"
			if hit {
				result = "hit"
			}
			c.metrics.RecordCacheOperation("get", result, latency, 0)
		}
	}()

	finalKey := c.buildKey(key)

	// Get from Redis
	data, err := c.client.Get(ctx, finalKey).Result()
	if err != nil {
		if err == redis.Nil {
			c.logger.Debug().Str("key", key).Msg("Cache miss")
			return nil, ErrCacheMiss
		}
		c.logger.Error().Err(err).Str("key", key).Msg("Failed to get cache value")
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	// Deserialize entry
	var entry CacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		c.logger.Error().Err(err).Str("key", key).Msg("Failed to unmarshal cache entry")
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	hit = true

	// Update access count asynchronously
	go func() {
		entry.AccessCount++
		if updatedData, err := json.Marshal(entry); err == nil {
			c.client.Set(context.Background(), finalKey, updatedData, time.Duration(entry.TTL)*time.Second)
		}
	}()

	c.logger.Debug().
		Str("key", key).
		Int64("access_count", entry.AccessCount).
		Msg("Cache hit")

	return entry.Value, nil
}

// Delete removes a value from cache
func (c *Cache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		c.updateStats(func(s *CacheStats) {
			s.Deletes++
			s.AverageLatency = (s.AverageLatency + latency) / 2
			s.LastUpdated = time.Now()
		})

		if c.metrics != nil {
			c.metrics.RecordCacheOperation("delete", "success", latency, 0)
		}
	}()

	finalKey := c.buildKey(key)

	count, err := c.client.Del(ctx, finalKey).Result()
	if err != nil {
		c.logger.Error().Err(err).Str("key", key).Msg("Failed to delete cache value")
		return fmt.Errorf("redis del error: %w", err)
	}

	if count > 0 {
		c.logger.Debug().Str("key", key).Msg("Cache value deleted")
	}

	return nil
}

// Exists checks if a key exists in cache
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	finalKey := c.buildKey(key)

	count, err := c.client.Exists(ctx, finalKey).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists error: %w", err)
	}

	return count > 0, nil
}

// updateStats safely updates cache statistics
func (c *Cache) updateStats(fn func(*CacheStats)) {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()
	fn(c.stats)

	// Calculate hit ratio
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRatio = float64(c.stats.Hits) / float64(total)
	}
}

// Stats returns current cache statistics
func (c *Cache) Stats() CacheStats {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	// Get current size from Redis
	if info, err := c.client.Info(context.Background(), "memory").Result(); err == nil {
		// Parse memory info (simplified)
		c.stats.Memory = parseMemoryInfo(info)
	}

	// Return a copy to avoid mutex issues
	return CacheStats{
		Hits:           c.stats.Hits,
		Misses:         c.stats.Misses,
		Sets:           c.stats.Sets,
		Deletes:        c.stats.Deletes,
		Evictions:      c.stats.Evictions,
		Size:           c.stats.Size,
		Memory:         c.stats.Memory,
		AverageLatency: c.stats.AverageLatency,
		HitRatio:       c.stats.HitRatio,
		LastUpdated:    c.stats.LastUpdated,
	}
}

// warmup preloads specified keys
func (c *Cache) warmup(ctx context.Context) {
	c.logger.Info().
		Int("keys", len(c.config.WarmupKeys)).
		Msg("Starting cache warmup")

	for _, key := range c.config.WarmupKeys {
		if _, err := c.Get(ctx, key); err != nil && err != ErrCacheMiss {
			c.logger.Warn().Err(err).Str("key", key).Msg("Warmup failed for key")
		}

		// Small delay between warmup requests
		time.Sleep(10 * time.Millisecond)
	}

	c.logger.Info().Msg("Cache warmup completed")
}

// Close closes the cache connection
func (c *Cache) Close() error {
	c.logger.Info().Msg("Closing cache connection")
	return c.client.Close()
}

// Helper functions

var ErrCacheMiss = fmt.Errorf("cache miss")

// estimateSize estimates the size of a value in bytes
func estimateSize(value interface{}) int64 {
	if data, err := json.Marshal(value); err == nil {
		return int64(len(data))
	}
	return 0
}

// parseMemoryInfo parses Redis memory info (simplified implementation)
func parseMemoryInfo(info string) int64 {
	// This is a simplified parser - in production you'd want proper parsing
	// For now, return 0 and let Redis handle memory tracking
	return 0
}
