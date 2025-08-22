package cache

import (
	"documents-worker/config"
	"log"
	"time"
)

// Manager manages the cache system
type Manager struct {
	fileCache *FileCache
	enabled   bool
}

// NewManager creates a new cache manager
func NewManager(cfg *config.Config) *Manager {
	if !cfg.Cache.Enabled {
		return &Manager{enabled: false}
	}

	fileCache := NewFileCache(
		cfg.Cache.Directory,
		cfg.Cache.TTL,
		cfg.Cache.MaxSize,
		cfg.Cache.CleanupAge,
	)

	manager := &Manager{
		fileCache: fileCache,
		enabled:   true,
	}

	// Start cleanup routine
	go manager.startCleanupRoutine()

	return manager
}

// IsEnabled returns whether caching is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// GetCachedResult retrieves a cached processing result
func (m *Manager) GetCachedResult(operation string, params ...string) (string, bool) {
	if !m.enabled {
		return "", false
	}

	key := m.fileCache.GenerateCacheKey(operation, params...)
	return m.fileCache.Get(key)
}

// CacheResult stores a processing result
func (m *Manager) CacheResult(operation string, resultPath string, params ...string) error {
	if !m.enabled {
		return nil
	}

	key := m.fileCache.GenerateCacheKey(operation, params...)
	return m.fileCache.Set(key, resultPath)
}

// InvalidateCache removes a cached result
func (m *Manager) InvalidateCache(operation string, params ...string) error {
	if !m.enabled {
		return nil
	}

	key := m.fileCache.GenerateCacheKey(operation, params...)
	return m.fileCache.Delete(key)
}

// GetCacheStats returns cache statistics
func (m *Manager) GetCacheStats() map[string]interface{} {
	if !m.enabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	size, _ := m.fileCache.Size()

	return map[string]interface{}{
		"enabled":   true,
		"size":      size,
		"directory": m.fileCache.directory,
		"ttl":       m.fileCache.ttl.String(),
		"max_size":  m.fileCache.maxSize,
	}
}

// startCleanupRoutine starts a background routine for cache cleanup
func (m *Manager) startCleanupRoutine() {
	if !m.enabled {
		return
	}

	ticker := time.NewTicker(1 * time.Hour) // Clean every hour
	defer ticker.Stop()

	for range ticker.C {
		// Clean expired files
		if err := m.fileCache.Clean(); err != nil {
			log.Printf("Cache cleanup error: %v", err)
		}

		// Enforce max size
		if err := m.fileCache.EnforceMaxSize(); err != nil {
			log.Printf("Cache size enforcement error: %v", err)
		}
	}
}
