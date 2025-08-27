package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// EventType represents the type of event
type EventType string

const (
	DocumentProcessedEvent EventType = "document.processed"
	DocumentFailedEvent    EventType = "document.failed"
	QueueJobCreatedEvent   EventType = "queue.job.created"
	QueueJobCompletedEvent EventType = "queue.job.completed"
	CacheHitEvent          EventType = "cache.hit"
	CacheMissEvent         EventType = "cache.miss"
	MemoryPoolGrowthEvent  EventType = "memory.pool.growth"
	MemoryPoolShrinkEvent  EventType = "memory.pool.shrink"
	HealthCheckFailedEvent EventType = "health.check.failed"
	RateLimitExceededEvent EventType = "rate.limit.exceeded"
)

// Event represents a system event
type Event struct {
	ID            string                 `json:"id"`
	Type          EventType              `json:"type"`
	Source        string                 `json:"source"`
	Timestamp     time.Time              `json:"timestamp"`
	Data          map[string]interface{} `json:"data"`
	Metadata      map[string]string      `json:"metadata,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Version       string                 `json:"version"`
}

// EventHandler defines the interface for handling events
type EventHandler interface {
	Handle(ctx context.Context, event *Event) error
	SupportedEvents() []EventType
}

// EventBus interface for event publishing and subscribing
type EventBus interface {
	Publish(ctx context.Context, event *Event) error
	Subscribe(eventType EventType, handler EventHandler) error
	Unsubscribe(eventType EventType, handler EventHandler) error
	Start(ctx context.Context) error
	Stop() error
}

// EventConfig holds event bus configuration
type EventConfig struct {
	RedisURL      string        `json:"redis_url" validate:"required"`
	StreamPrefix  string        `json:"stream_prefix"`
	ConsumerGroup string        `json:"consumer_group"`
	ConsumerName  string        `json:"consumer_name"`
	MaxRetries    int           `json:"max_retries" validate:"min=1,max=10"`
	RetryDelay    time.Duration `json:"retry_delay" validate:"min=100ms"`
	BatchSize     int           `json:"batch_size" validate:"min=1,max=100"`
	BlockTimeout  time.Duration `json:"block_timeout" validate:"min=1s"`
	EnableMetrics bool          `json:"enable_metrics"`
	BufferSize    int           `json:"buffer_size" validate:"min=1,max=1000"`
}

// DefaultEventConfig returns default event configuration
func DefaultEventConfig() *EventConfig {
	return &EventConfig{
		RedisURL:      "redis://localhost:6379",
		StreamPrefix:  "docworker:events",
		ConsumerGroup: "docworker-processors",
		ConsumerName:  "docworker-1",
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
		BatchSize:     10,
		BlockTimeout:  5 * time.Second,
		EnableMetrics: true,
		BufferSize:    100,
	}
}

// RedisEventBus implements EventBus using Redis Streams
type RedisEventBus struct {
	client   *redis.Client
	config   *EventConfig
	logger   zerolog.Logger
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	metrics  EventMetrics
}

// EventMetrics interface for event metrics
type EventMetrics interface {
	RecordEventPublished(eventType string, success bool, latency time.Duration)
	RecordEventProcessed(eventType string, success bool, latency time.Duration)
	RecordEventHandlerError(eventType string, handler string, err error)
}

// NewRedisEventBus creates a new Redis-based event bus
func NewRedisEventBus(config *EventConfig, logger zerolog.Logger, metrics EventMetrics) (*RedisEventBus, error) {
	if config == nil {
		config = DefaultEventConfig()
	}

	// Parse Redis URL
	opt, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	ctx, cancel = context.WithCancel(context.Background())

	bus := &RedisEventBus{
		client:   client,
		config:   config,
		logger:   logger.With().Str("component", "event_bus").Logger(),
		handlers: make(map[EventType][]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
		metrics:  metrics,
	}

	bus.logger.Info().
		Str("redis_url", config.RedisURL).
		Str("stream_prefix", config.StreamPrefix).
		Str("consumer_group", config.ConsumerGroup).
		Msg("Event bus initialized")

	return bus, nil
}

// Publish publishes an event to the event bus
func (b *RedisEventBus) Publish(ctx context.Context, event *Event) error {
	start := time.Now()
	var success bool

	defer func() {
		latency := time.Since(start)
		if b.metrics != nil {
			b.metrics.RecordEventPublished(string(event.Type), success, latency)
		}
	}()

	// Set event metadata
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d_%s", time.Now().UnixNano(), event.Type)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Version == "" {
		event.Version = "1.0"
	}

	// Serialize event
	data, err := json.Marshal(event)
	if err != nil {
		b.logger.Error().Err(err).Str("event_id", event.ID).Msg("Failed to marshal event")
		return fmt.Errorf("marshal event: %w", err)
	}

	// Create stream name
	streamName := fmt.Sprintf("%s:%s", b.config.StreamPrefix, event.Type)

	// Publish to Redis Stream
	args := &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"event_id":   event.ID,
			"event_type": event.Type,
			"source":     event.Source,
			"data":       string(data),
			"timestamp":  event.Timestamp.Unix(),
		},
	}

	_, err = b.client.XAdd(ctx, args).Result()
	if err != nil {
		b.logger.Error().Err(err).
			Str("event_id", event.ID).
			Str("stream", streamName).
			Msg("Failed to publish event")
		return fmt.Errorf("publish to stream: %w", err)
	}

	success = true
	b.logger.Debug().
		Str("event_id", event.ID).
		Str("event_type", string(event.Type)).
		Str("stream", streamName).
		Msg("Event published")

	return nil
}

// Subscribe subscribes a handler to specific event types
func (b *RedisEventBus) Subscribe(eventType EventType, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.handlers[eventType] == nil {
		b.handlers[eventType] = make([]EventHandler, 0)
	}

	b.handlers[eventType] = append(b.handlers[eventType], handler)

	b.logger.Info().
		Str("event_type", string(eventType)).
		Int("handler_count", len(b.handlers[eventType])).
		Msg("Event handler subscribed")

	return nil
}

// Unsubscribe removes a handler from event type subscriptions
func (b *RedisEventBus) Unsubscribe(eventType EventType, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers := b.handlers[eventType]
	for i, h := range handlers {
		if h == handler {
			// Remove handler from slice
			b.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			b.logger.Info().
				Str("event_type", string(eventType)).
				Int("remaining_handlers", len(b.handlers[eventType])).
				Msg("Event handler unsubscribed")
			return nil
		}
	}

	return fmt.Errorf("handler not found for event type: %s", eventType)
}

// Start starts the event bus and begins processing events
func (b *RedisEventBus) Start(ctx context.Context) error {
	b.logger.Info().Msg("Starting event bus")

	// Create consumer groups for all subscribed event types
	for eventType := range b.handlers {
		if err := b.createConsumerGroup(eventType); err != nil {
			b.logger.Warn().Err(err).
				Str("event_type", string(eventType)).
				Msg("Failed to create consumer group, may already exist")
		}
	}

	// Start consumer goroutines for each event type
	for eventType := range b.handlers {
		b.wg.Add(1)
		go b.consumeEvents(eventType)
	}

	b.logger.Info().
		Int("event_types", len(b.handlers)).
		Msg("Event bus started")

	return nil
}

// Stop stops the event bus
func (b *RedisEventBus) Stop() error {
	b.logger.Info().Msg("Stopping event bus")

	b.cancel()
	b.wg.Wait()

	if err := b.client.Close(); err != nil {
		b.logger.Error().Err(err).Msg("Failed to close Redis client")
		return err
	}

	b.logger.Info().Msg("Event bus stopped")
	return nil
}

// createConsumerGroup creates a consumer group for an event type
func (b *RedisEventBus) createConsumerGroup(eventType EventType) error {
	streamName := fmt.Sprintf("%s:%s", b.config.StreamPrefix, eventType)

	err := b.client.XGroupCreate(context.Background(), streamName, b.config.ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group: %w", err)
	}

	return nil
}

// consumeEvents consumes events for a specific event type
func (b *RedisEventBus) consumeEvents(eventType EventType) {
	defer b.wg.Done()

	streamName := fmt.Sprintf("%s:%s", b.config.StreamPrefix, eventType)

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
			// Read events from stream
			streams, err := b.client.XReadGroup(b.ctx, &redis.XReadGroupArgs{
				Group:    b.config.ConsumerGroup,
				Consumer: b.config.ConsumerName,
				Streams:  []string{streamName, ">"},
				Count:    int64(b.config.BatchSize),
				Block:    b.config.BlockTimeout,
			}).Result()

			if err != nil {
				if err != redis.Nil && err != context.Canceled {
					b.logger.Error().Err(err).
						Str("stream", streamName).
						Msg("Failed to read from stream")
				}
				continue
			}

			// Process each stream
			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := b.processMessage(eventType, message); err != nil {
						b.logger.Error().Err(err).
							Str("message_id", message.ID).
							Str("stream", streamName).
							Msg("Failed to process message")
					} else {
						// Acknowledge the message
						b.client.XAck(b.ctx, streamName, b.config.ConsumerGroup, message.ID)
					}
				}
			}
		}
	}
}

// processMessage processes a single event message
func (b *RedisEventBus) processMessage(eventType EventType, message redis.XMessage) error {
	start := time.Now()
	var success bool

	defer func() {
		latency := time.Since(start)
		if b.metrics != nil {
			b.metrics.RecordEventProcessed(string(eventType), success, latency)
		}
	}()

	// Extract event data
	eventData, ok := message.Values["data"].(string)
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Deserialize event
	var event Event
	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	// Get handlers for this event type
	b.mu.RLock()
	handlers := b.handlers[eventType]
	b.mu.RUnlock()

	// Process event with all registered handlers
	for _, handler := range handlers {
		if err := handler.Handle(b.ctx, &event); err != nil {
			if b.metrics != nil {
				b.metrics.RecordEventHandlerError(string(eventType), fmt.Sprintf("%T", handler), err)
			}
			b.logger.Error().Err(err).
				Str("event_id", event.ID).
				Str("handler", fmt.Sprintf("%T", handler)).
				Msg("Event handler failed")
			return err
		}
	}

	success = true
	b.logger.Debug().
		Str("event_id", event.ID).
		Str("event_type", string(eventType)).
		Int("handlers", len(handlers)).
		Msg("Event processed successfully")

	return nil
}

// Helper functions for creating events

// NewDocumentProcessedEvent creates a document processed event
func NewDocumentProcessedEvent(source, documentID string, metadata map[string]string) *Event {
	return &Event{
		Type:   DocumentProcessedEvent,
		Source: source,
		Data: map[string]interface{}{
			"document_id": documentID,
			"status":      "completed",
		},
		Metadata: metadata,
	}
}

// NewDocumentFailedEvent creates a document failed event
func NewDocumentFailedEvent(source, documentID string, err error, metadata map[string]string) *Event {
	return &Event{
		Type:   DocumentFailedEvent,
		Source: source,
		Data: map[string]interface{}{
			"document_id": documentID,
			"error":       err.Error(),
			"status":      "failed",
		},
		Metadata: metadata,
	}
}

// NewMemoryPoolEvent creates a memory pool event
func NewMemoryPoolEvent(eventType EventType, source string, poolStats map[string]interface{}) *Event {
	return &Event{
		Type:   eventType,
		Source: source,
		Data:   poolStats,
	}
}

// NewCacheEvent creates a cache event
func NewCacheEvent(eventType EventType, source, key string, hit bool) *Event {
	return &Event{
		Type:   eventType,
		Source: source,
		Data: map[string]interface{}{
			"cache_key": key,
			"hit":       hit,
		},
	}
}
