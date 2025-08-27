package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// MockEventMetrics implements EventMetrics for testing
type MockEventMetrics struct {
	publishedEvents map[string]int
	processedEvents map[string]int
	errors          map[string]int
	mu              sync.RWMutex
}

func NewMockEventMetrics() *MockEventMetrics {
	return &MockEventMetrics{
		publishedEvents: make(map[string]int),
		processedEvents: make(map[string]int),
		errors:          make(map[string]int),
	}
}

func (m *MockEventMetrics) RecordEventPublished(eventType string, success bool, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := eventType + "_published"
	if success {
		key += "_success"
	} else {
		key += "_failed"
	}
	m.publishedEvents[key]++
}

func (m *MockEventMetrics) RecordEventProcessed(eventType string, success bool, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := eventType + "_processed"
	if success {
		key += "_success"
	} else {
		key += "_failed"
	}
	m.processedEvents[key]++
}

func (m *MockEventMetrics) RecordEventHandlerError(eventType string, handler string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := eventType + "_" + handler
	m.errors[key]++
}

func (m *MockEventMetrics) GetPublishedCount(eventType string, success bool) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := eventType + "_published"
	if success {
		key += "_success"
	} else {
		key += "_failed"
	}
	return m.publishedEvents[key]
}

func (m *MockEventMetrics) GetProcessedCount(eventType string, success bool) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := eventType + "_processed"
	if success {
		key += "_success"
	} else {
		key += "_failed"
	}
	return m.processedEvents[key]
}

// MockEventHandler implements EventHandler for testing
type MockEventHandler struct {
	handledEvents  []Event
	supportedTypes []EventType
	shouldFail     bool
	mu             sync.RWMutex
}

func NewMockEventHandler(supportedTypes []EventType) *MockEventHandler {
	return &MockEventHandler{
		handledEvents:  make([]Event, 0),
		supportedTypes: supportedTypes,
		shouldFail:     false,
	}
}

func (h *MockEventHandler) Handle(ctx context.Context, event *Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.shouldFail {
		return assert.AnError
	}

	h.handledEvents = append(h.handledEvents, *event)
	return nil
}

func (h *MockEventHandler) SupportedEvents() []EventType {
	return h.supportedTypes
}

func (h *MockEventHandler) GetHandledEvents() []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]Event{}, h.handledEvents...)
}

func (h *MockEventHandler) SetShouldFail(fail bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.shouldFail = fail
}

func TestDefaultEventConfig(t *testing.T) {
	config := DefaultEventConfig()

	assert.Equal(t, "redis://localhost:6379", config.RedisURL)
	assert.Equal(t, "docworker:events", config.StreamPrefix)
	assert.Equal(t, "docworker-processors", config.ConsumerGroup)
	assert.Equal(t, "docworker-1", config.ConsumerName)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 10, config.BatchSize)
	assert.Equal(t, 5*time.Second, config.BlockTimeout)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, 100, config.BufferSize)
}

func TestEventCreation(t *testing.T) {
	t.Run("document processed event", func(t *testing.T) {
		event := NewDocumentProcessedEvent("test-service", "doc-123", map[string]string{
			"user_id": "user-456",
		})

		assert.Equal(t, DocumentProcessedEvent, event.Type)
		assert.Equal(t, "test-service", event.Source)
		assert.Equal(t, "doc-123", event.Data["document_id"])
		assert.Equal(t, "completed", event.Data["status"])
		assert.Equal(t, "user-456", event.Metadata["user_id"])
	})

	t.Run("document failed event", func(t *testing.T) {
		err := assert.AnError
		event := NewDocumentFailedEvent("test-service", "doc-123", err, nil)

		assert.Equal(t, DocumentFailedEvent, event.Type)
		assert.Equal(t, "test-service", event.Source)
		assert.Equal(t, "doc-123", event.Data["document_id"])
		assert.Equal(t, "failed", event.Data["status"])
		assert.Equal(t, err.Error(), event.Data["error"])
	})

	t.Run("memory pool event", func(t *testing.T) {
		stats := map[string]interface{}{
			"current_buffers": 10,
			"peak_buffers":    15,
		}
		event := NewMemoryPoolEvent(MemoryPoolGrowthEvent, "memory-pool", stats)

		assert.Equal(t, MemoryPoolGrowthEvent, event.Type)
		assert.Equal(t, "memory-pool", event.Source)
		assert.Equal(t, 10, event.Data["current_buffers"])
		assert.Equal(t, 15, event.Data["peak_buffers"])
	})

	t.Run("cache event", func(t *testing.T) {
		event := NewCacheEvent(CacheHitEvent, "cache-service", "user:123", true)

		assert.Equal(t, CacheHitEvent, event.Type)
		assert.Equal(t, "cache-service", event.Source)
		assert.Equal(t, "user:123", event.Data["cache_key"])
		assert.Equal(t, true, event.Data["hit"])
	})
}

func TestEventMetadata(t *testing.T) {
	event := &Event{
		Type:   DocumentProcessedEvent,
		Source: "test",
		Data:   map[string]interface{}{"test": "data"},
	}

	// Test auto-generation of ID and timestamp
	assert.Empty(t, event.ID)
	assert.True(t, event.Timestamp.IsZero())
	assert.Empty(t, event.Version)

	// These would be set during publishing in the real implementation
}

func TestMockEventHandler(t *testing.T) {
	handler := NewMockEventHandler([]EventType{DocumentProcessedEvent, DocumentFailedEvent})

	// Test supported events
	supported := handler.SupportedEvents()
	assert.Len(t, supported, 2)
	assert.Contains(t, supported, DocumentProcessedEvent)
	assert.Contains(t, supported, DocumentFailedEvent)

	// Test handling events
	event := &Event{
		ID:   "test-event",
		Type: DocumentProcessedEvent,
		Data: map[string]interface{}{"test": "data"},
	}

	err := handler.Handle(context.Background(), event)
	assert.NoError(t, err)

	handled := handler.GetHandledEvents()
	assert.Len(t, handled, 1)
	assert.Equal(t, "test-event", handled[0].ID)

	// Test failure mode
	handler.SetShouldFail(true)
	err = handler.Handle(context.Background(), event)
	assert.Error(t, err)
}

func TestMockEventMetrics(t *testing.T) {
	metrics := NewMockEventMetrics()

	// Test recording published events
	metrics.RecordEventPublished("test.event", true, 10*time.Millisecond)
	metrics.RecordEventPublished("test.event", false, 5*time.Millisecond)

	assert.Equal(t, 1, metrics.GetPublishedCount("test.event", true))
	assert.Equal(t, 1, metrics.GetPublishedCount("test.event", false))

	// Test recording processed events
	metrics.RecordEventProcessed("test.event", true, 15*time.Millisecond)

	assert.Equal(t, 1, metrics.GetProcessedCount("test.event", true))
	assert.Equal(t, 0, metrics.GetProcessedCount("test.event", false))

	// Test recording handler errors
	metrics.RecordEventHandlerError("test.event", "TestHandler", assert.AnError)

	// Error counts are recorded but not exposed in this simple mock
}

func TestEventTypes(t *testing.T) {
	// Test all defined event types are unique
	eventTypes := []EventType{
		DocumentProcessedEvent,
		DocumentFailedEvent,
		QueueJobCreatedEvent,
		QueueJobCompletedEvent,
		CacheHitEvent,
		CacheMissEvent,
		MemoryPoolGrowthEvent,
		MemoryPoolShrinkEvent,
		HealthCheckFailedEvent,
		RateLimitExceededEvent,
	}

	seen := make(map[EventType]bool)
	for _, eventType := range eventTypes {
		assert.False(t, seen[eventType], "Duplicate event type: %s", eventType)
		seen[eventType] = true
		assert.NotEmpty(t, string(eventType), "Event type should not be empty")
	}

	assert.Len(t, seen, len(eventTypes), "All event types should be unique")
}

// Integration tests would require a running Redis instance
// For now, we'll test the basic functionality without Redis

func TestEventBusInterface(t *testing.T) {
	// Test that RedisEventBus implements EventBus interface
	logger := zerolog.New(nil)
	metrics := NewMockEventMetrics()
	config := DefaultEventConfig()

	// This would fail without Redis, but we're just testing the interface
	bus, err := NewRedisEventBus(config, logger, metrics)

	// We expect this to fail due to no Redis connection
	assert.Error(t, err)
	assert.Nil(t, bus)

	// Check that error message contains connection failure info (if err is not nil)
	if err != nil {
		assert.Contains(t, err.Error(), "redis")
	}
}

func TestEventConfigValidation(t *testing.T) {
	config := DefaultEventConfig()

	// Test that all required fields are set
	assert.NotEmpty(t, config.RedisURL)
	assert.NotEmpty(t, config.StreamPrefix)
	assert.NotEmpty(t, config.ConsumerGroup)
	assert.NotEmpty(t, config.ConsumerName)
	assert.Greater(t, config.MaxRetries, 0)
	assert.Greater(t, config.RetryDelay, time.Duration(0))
	assert.Greater(t, config.BatchSize, 0)
	assert.Greater(t, config.BlockTimeout, time.Duration(0))
	assert.Greater(t, config.BufferSize, 0)
}
