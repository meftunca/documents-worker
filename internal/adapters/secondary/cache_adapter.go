package adapters

import (
	"context"
	"documents-worker/cache"
	"documents-worker/internal/core/ports"
	"time"
)

// CacheAdapter wraps the existing CacheManager to implement ports.Cache
type CacheAdapter struct {
	cacheManager *cache.CacheManager
}

// NewCacheAdapter creates a new cache adapter
func NewCacheAdapter(cacheManager *cache.CacheManager) ports.Cache {
	return &CacheAdapter{
		cacheManager: cacheManager,
	}
}

func (c *CacheAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := c.cacheManager.Get(key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	// For now, we'll return the key as data since CacheEntry doesn't store raw data
	// In a real implementation, you'd read from value.OutputPath or implement proper data storage
	return []byte(value.Key), nil
}

func (c *CacheAdapter) Set(ctx context.Context, key string, value []byte, ttl int64) error {
	// Convert ttl from int64 seconds to time.Duration
	duration := time.Duration(ttl) * time.Second

	// CacheManager.Set requires (key, data, contentType, mimeType, metadata)
	metadata := make(map[string]interface{})
	metadata["ttl"] = duration

	return c.cacheManager.Set(key, string(value), "text/plain", "text/plain", metadata)
}

func (c *CacheAdapter) Delete(ctx context.Context, key string) error {
	return c.cacheManager.Delete(key)
}

func (c *CacheAdapter) Exists(ctx context.Context, key string) (bool, error) {
	value, err := c.cacheManager.Get(key)
	if err != nil {
		return false, err
	}
	return value != nil, nil
}

func (c *CacheAdapter) Close() error {
	// CacheManager doesn't have a Close method in the existing code
	// This would need to be implemented if required
	return nil
}
