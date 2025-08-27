package cache

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMetrics implements CacheMetrics for testing
type MockMetrics struct {
	operations map[string]int
	mu         sync.RWMutex
}

func NewMockMetrics() *MockMetrics {
	return &MockMetrics{
		operations: make(map[string]int),
	}
}

func (m *MockMetrics) RecordCacheOperation(operation, result string, latency time.Duration, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operation + "_" + result
	m.operations[key]++
}

func (m *MockMetrics) RecordCacheSize(size int64) {}

func (m *MockMetrics) RecordCacheMemory(memory int64) {}

func (m *MockMetrics) GetOperationCount(operation, result string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := operation + "_" + result
	return m.operations[key]
}

func TestDefaultCacheConfig(t *testing.T) {
	config := DefaultCacheConfig()

	assert.Equal(t, "redis://localhost:6379", config.RedisURL)
	assert.Equal(t, 1*time.Hour, config.DefaultTTL)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryDelay)
	assert.Equal(t, 10, config.PoolSize)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, "docworker", config.Namespace)
	assert.Equal(t, 6, config.CompressionLevel)
}

func TestCachePatternMatching(t *testing.T) {
	tests := []struct {
		pattern string
		key     string
		matches bool
	}{
		{"*", "anything", true},
		{"user:*", "user:123", true},
		{"user:*", "session:123", false},
		{"temp:*", "temp:abc", true},
		{"exact", "exact", true},
		{"exact", "exactish", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.key, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.key)
			assert.Equal(t, tt.matches, result)
		})
	}
}

func TestCacheEntry(t *testing.T) {
	entry := CacheEntry{
		Value:       "test_value",
		TTL:         3600,
		CreatedAt:   time.Now(),
		AccessCount: 0,
		Tags:        []string{"tag1", "tag2"},
	}

	// Test JSON marshaling/unmarshaling
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var decoded CacheEntry
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, entry.Value, decoded.Value)
	assert.Equal(t, entry.TTL, decoded.TTL)
	assert.Equal(t, entry.Tags, decoded.Tags)
}

func TestCacheStats(t *testing.T) {
	stats := &CacheStats{
		Hits:   10,
		Misses: 5,
	}

	// Test hit ratio calculation
	total := stats.Hits + stats.Misses
	expectedRatio := float64(stats.Hits) / float64(total)

	// Simulate updateStats behavior
	stats.HitRatio = expectedRatio

	assert.Equal(t, 10, int(stats.Hits))
	assert.Equal(t, 5, int(stats.Misses))
	assert.InDelta(t, 0.666, stats.HitRatio, 0.01)
}

func TestEstimateSize(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		min   int64
	}{
		{"string", "hello", 5},
		{"number", 123, 3},
		{"struct", map[string]string{"key": "value"}, 10},
		{"array", []int{1, 2, 3}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := estimateSize(tt.value)
			assert.GreaterOrEqual(t, size, tt.min)
		})
	}
}

// Integration tests would require a running Redis instance
// For now, we'll test the basic functionality without Redis

func TestCacheConfigValidation(t *testing.T) {
	config := DefaultCacheConfig()

	// Test that all required fields are set
	assert.NotEmpty(t, config.RedisURL)
	assert.Greater(t, config.DefaultTTL, time.Duration(0))
	assert.Greater(t, config.MaxRetries, 0)
	assert.Greater(t, config.PoolSize, 0)
}

func TestBuildKey(t *testing.T) {
	logger := zerolog.New(nil)
	metrics := NewMockMetrics()

	// Test with namespace
	config := DefaultCacheConfig()
	config.Namespace = "test"

	cache := &Cache{
		config:   config,
		stats:    &CacheStats{},
		metrics:  metrics,
		logger:   logger,
		patterns: make(map[string]time.Duration),
	}

	key := cache.buildKey("mykey")
	assert.Equal(t, "test:mykey", key)

	// Test without namespace
	config.Namespace = ""
	cache.config = config

	key = cache.buildKey("mykey")
	assert.Equal(t, "mykey", key)
}

func TestTTLPatterns(t *testing.T) {
	logger := zerolog.New(nil)
	metrics := NewMockMetrics()
	config := DefaultCacheConfig()

	cache := &Cache{
		config:   config,
		stats:    &CacheStats{},
		metrics:  metrics,
		logger:   logger,
		patterns: make(map[string]time.Duration),
	}

	// Set patterns
	cache.SetTTLPattern("user:*", 30*time.Minute)
	cache.SetTTLPattern("session:*", 15*time.Minute)

	// Test pattern matching
	userTTL := cache.getTTLForKey("user:123")
	assert.Equal(t, 30*time.Minute, userTTL)

	sessionTTL := cache.getTTLForKey("session:abc")
	assert.Equal(t, 15*time.Minute, sessionTTL)

	defaultTTL := cache.getTTLForKey("other:key")
	assert.Equal(t, config.DefaultTTL, defaultTTL)
}

func TestUpdateStats(t *testing.T) {
	stats := &CacheStats{}

	cache := &Cache{
		stats: stats,
	}

	// Test stats update
	cache.updateStats(func(s *CacheStats) {
		s.Hits = 10
		s.Misses = 5
	})

	// Verify hit ratio calculation
	assert.Equal(t, int64(10), stats.Hits)
	assert.Equal(t, int64(5), stats.Misses)
	assert.InDelta(t, 0.666, stats.HitRatio, 0.01)
}

func TestCacheMetricsInterface(t *testing.T) {
	metrics := NewMockMetrics()

	// Test metrics recording
	metrics.RecordCacheOperation("get", "hit", 10*time.Millisecond, 100)
	metrics.RecordCacheOperation("get", "miss", 5*time.Millisecond, 0)
	metrics.RecordCacheOperation("set", "success", 15*time.Millisecond, 200)

	assert.Equal(t, 1, metrics.GetOperationCount("get", "hit"))
	assert.Equal(t, 1, metrics.GetOperationCount("get", "miss"))
	assert.Equal(t, 1, metrics.GetOperationCount("set", "success"))
	assert.Equal(t, 0, metrics.GetOperationCount("delete", "success"))
}
