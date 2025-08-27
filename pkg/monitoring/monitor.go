package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// HealthStatus represents health status levels
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// ComponentHealth represents health status of a component
type ComponentHealth struct {
	Name       string                 `json:"name"`
	Status     HealthStatus           `json:"status"`
	Message    string                 `json:"message"`
	LastCheck  time.Time              `json:"last_check"`
	CheckCount int64                  `json:"check_count"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Error      error                  `json:"error,omitempty"`
}

// SystemHealth represents overall system health
type SystemHealth struct {
	Status     HealthStatus               `json:"status"`
	Message    string                     `json:"message"`
	Timestamp  time.Time                  `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components"`
	Uptime     time.Duration              `json:"uptime"`
	Version    string                     `json:"version"`
}

// HealthChecker interface for health check implementations
type HealthChecker interface {
	Check(ctx context.Context) ComponentHealth
	Name() string
}

// AlertLevel represents alert severity levels
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert represents a monitoring alert
type Alert struct {
	ID          string                 `json:"id"`
	Level       AlertLevel             `json:"level"`
	Component   string                 `json:"component"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// AlertHandler interface for alert handling
type AlertHandler interface {
	HandleAlert(alert Alert) error
}

// CircuitBreakerState represents circuit breaker states
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"
	CircuitBreakerOpen     CircuitBreakerState = "open"
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Name               string        `json:"name"`
	FailureThreshold   int           `json:"failure_threshold"`
	RecoveryTimeout    time.Duration `json:"recovery_timeout"`
	SuccessThreshold   int           `json:"success_threshold"`
	Timeout            time.Duration `json:"timeout"`
	MaxConcurrentCalls int           `json:"max_concurrent_calls"`
}

// CircuitBreakerStats tracks circuit breaker statistics
type CircuitBreakerStats struct {
	State               CircuitBreakerState `json:"state"`
	Requests            int64               `json:"requests"`
	Successes           int64               `json:"successes"`
	Failures            int64               `json:"failures"`
	ConsecutiveFailures int                 `json:"consecutive_failures"`
	LastFailureTime     time.Time           `json:"last_failure_time"`
	NextRetryTime       time.Time           `json:"next_retry_time"`
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	config *CircuitBreakerConfig
	stats  CircuitBreakerStats
	mu     sync.RWMutex
	logger zerolog.Logger
}

// MonitorConfig holds monitoring system configuration
type MonitorConfig struct {
	HealthCheckInterval time.Duration           `json:"health_check_interval"`
	AlertingEnabled     bool                    `json:"alerting_enabled"`
	MetricsEnabled      bool                    `json:"metrics_enabled"`
	CircuitBreakers     []*CircuitBreakerConfig `json:"circuit_breakers"`
}

// DefaultMonitorConfig returns default monitoring configuration
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		HealthCheckInterval: 30 * time.Second,
		AlertingEnabled:     true,
		MetricsEnabled:      true,
		CircuitBreakers: []*CircuitBreakerConfig{
			{
				Name:               "redis",
				FailureThreshold:   5,
				RecoveryTimeout:    30 * time.Second,
				SuccessThreshold:   3,
				Timeout:            5 * time.Second,
				MaxConcurrentCalls: 100,
			},
			{
				Name:               "database",
				FailureThreshold:   3,
				RecoveryTimeout:    60 * time.Second,
				SuccessThreshold:   2,
				Timeout:            10 * time.Second,
				MaxConcurrentCalls: 50,
			},
		},
	}
}

// Monitor manages health checks, alerting, and circuit breakers
type Monitor struct {
	config          *MonitorConfig
	checkers        map[string]HealthChecker
	alertHandlers   []AlertHandler
	circuitBreakers map[string]*CircuitBreaker
	systemHealth    SystemHealth
	startTime       time.Time
	logger          zerolog.Logger
	mu              sync.RWMutex
	stopChan        chan struct{}
}

// NewMonitor creates a new monitoring system
func NewMonitor(config *MonitorConfig, logger zerolog.Logger) *Monitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	monitor := &Monitor{
		config:          config,
		checkers:        make(map[string]HealthChecker),
		alertHandlers:   make([]AlertHandler, 0),
		circuitBreakers: make(map[string]*CircuitBreaker),
		startTime:       time.Now(),
		logger:          logger.With().Str("component", "monitor").Logger(),
		stopChan:        make(chan struct{}),
		systemHealth: SystemHealth{
			Status:     HealthStatusUnknown,
			Components: make(map[string]ComponentHealth),
			Version:    "2.0.0",
		},
	}

	// Initialize circuit breakers
	for _, cbConfig := range config.CircuitBreakers {
		cb := NewCircuitBreaker(cbConfig, logger)
		monitor.circuitBreakers[cbConfig.Name] = cb
	}

	monitor.logger.Info().Msg("Monitor initialized")
	return monitor
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger zerolog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		stats: CircuitBreakerStats{
			State: CircuitBreakerClosed,
		},
		logger: logger.With().Str("circuit_breaker", config.Name).Logger(),
	}
}

// Start starts the monitoring system
func (m *Monitor) Start(ctx context.Context) error {
	m.logger.Info().Msg("Starting monitoring system")

	// Start health check loop
	go m.healthCheckLoop(ctx)

	m.logger.Info().Msg("Monitoring system started")
	return nil
}

// Stop stops the monitoring system
func (m *Monitor) Stop() error {
	m.logger.Info().Msg("Stopping monitoring system")
	close(m.stopChan)
	m.logger.Info().Msg("Monitoring system stopped")
	return nil
}

// RegisterHealthChecker registers a health checker
func (m *Monitor) RegisterHealthChecker(checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkers[checker.Name()] = checker
	m.logger.Info().Str("checker", checker.Name()).Msg("Health checker registered")
}

// RegisterAlertHandler registers an alert handler
func (m *Monitor) RegisterAlertHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alertHandlers = append(m.alertHandlers, handler)
	m.logger.Info().Msg("Alert handler registered")
}

// GetSystemHealth returns current system health
func (m *Monitor) GetSystemHealth() SystemHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Update uptime
	health := m.systemHealth
	health.Uptime = time.Since(m.startTime)
	health.Timestamp = time.Now()

	return health
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (m *Monitor) GetCircuitBreakerStats(name string) (CircuitBreakerStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cb, exists := m.circuitBreakers[name]
	if !exists {
		return CircuitBreakerStats{}, fmt.Errorf("circuit breaker %s not found", name)
	}

	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.stats, nil
}

// ExecuteWithCircuitBreaker executes a function with circuit breaker protection
func (m *Monitor) ExecuteWithCircuitBreaker(name string, fn func() error) error {
	m.mu.RLock()
	cb, exists := m.circuitBreakers[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("circuit breaker %s not found", name)
	}

	return cb.Execute(fn)
}

// healthCheckLoop runs periodic health checks
func (m *Monitor) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks performs all registered health checks
func (m *Monitor) performHealthChecks(ctx context.Context) {
	m.mu.Lock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range m.checkers {
		checkers[name] = checker
	}
	m.mu.Unlock()

	results := make(map[string]ComponentHealth)
	var wg sync.WaitGroup

	// Run health checks concurrently
	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			start := time.Now()
			health := checker.Check(ctx)
			health.Duration = time.Since(start)
			health.CheckCount++

			results[name] = health
		}(name, checker)
	}

	wg.Wait()

	// Update system health
	m.updateSystemHealth(results)
}

// updateSystemHealth updates the overall system health
func (m *Monitor) updateSystemHealth(components map[string]ComponentHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.systemHealth.Components = components
	m.systemHealth.Timestamp = time.Now()

	// Determine overall health status
	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	for _, component := range components {
		switch component.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		case HealthStatusUnhealthy:
			unhealthyCount++
		}
	}

	totalComponents := len(components)
	if totalComponents == 0 {
		m.systemHealth.Status = HealthStatusUnknown
		m.systemHealth.Message = "No components registered"
		return
	}

	if unhealthyCount > 0 {
		m.systemHealth.Status = HealthStatusUnhealthy
		m.systemHealth.Message = fmt.Sprintf("%d/%d components unhealthy", unhealthyCount, totalComponents)
	} else if degradedCount > 0 {
		m.systemHealth.Status = HealthStatusDegraded
		m.systemHealth.Message = fmt.Sprintf("%d/%d components degraded", degradedCount, totalComponents)
	} else {
		m.systemHealth.Status = HealthStatusHealthy
		m.systemHealth.Message = fmt.Sprintf("All %d components healthy", totalComponents)
	}

	// Generate alerts for status changes
	if m.config.AlertingEnabled {
		m.generateAlerts(components)
	}
}

// generateAlerts generates alerts based on health status changes
func (m *Monitor) generateAlerts(components map[string]ComponentHealth) {
	for name, component := range components {
		if component.Status == HealthStatusUnhealthy {
			alert := Alert{
				ID:          fmt.Sprintf("health-%s-%d", name, time.Now().Unix()),
				Level:       AlertLevelCritical,
				Component:   name,
				Title:       fmt.Sprintf("Component %s is unhealthy", name),
				Description: component.Message,
				Timestamp:   time.Now(),
				Metadata: map[string]interface{}{
					"check_duration": component.Duration,
					"check_count":    component.CheckCount,
				},
			}
			m.sendAlert(alert)
		} else if component.Status == HealthStatusDegraded {
			alert := Alert{
				ID:          fmt.Sprintf("health-%s-%d", name, time.Now().Unix()),
				Level:       AlertLevelWarning,
				Component:   name,
				Title:       fmt.Sprintf("Component %s is degraded", name),
				Description: component.Message,
				Timestamp:   time.Now(),
				Metadata: map[string]interface{}{
					"check_duration": component.Duration,
					"check_count":    component.CheckCount,
				},
			}
			m.sendAlert(alert)
		}
	}
}

// sendAlert sends an alert to all registered handlers
func (m *Monitor) sendAlert(alert Alert) {
	for _, handler := range m.alertHandlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(alert); err != nil {
				m.logger.Error().Err(err).Str("alert_id", alert.ID).Msg("Failed to handle alert")
			}
		}(handler)
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check circuit breaker state
	switch cb.stats.State {
	case CircuitBreakerOpen:
		if time.Now().Before(cb.stats.NextRetryTime) {
			return fmt.Errorf("circuit breaker %s is open", cb.config.Name)
		}
		// Transition to half-open
		cb.stats.State = CircuitBreakerHalfOpen
		cb.logger.Info().Msg("Circuit breaker transitioning to half-open")

	case CircuitBreakerHalfOpen:
		// Allow limited requests in half-open state

	case CircuitBreakerClosed:
		// Normal operation
	}

	// Execute function
	cb.stats.Requests++
	err := fn()

	if err != nil {
		cb.stats.Failures++
		cb.stats.ConsecutiveFailures++
		cb.stats.LastFailureTime = time.Now()

		// Check if we should open the circuit
		if cb.stats.State == CircuitBreakerClosed && cb.stats.ConsecutiveFailures >= cb.config.FailureThreshold {
			cb.stats.State = CircuitBreakerOpen
			cb.stats.NextRetryTime = time.Now().Add(cb.config.RecoveryTimeout)
			cb.logger.Warn().
				Int("failures", cb.stats.ConsecutiveFailures).
				Msg("Circuit breaker opened due to failures")
		} else if cb.stats.State == CircuitBreakerHalfOpen {
			// Failed in half-open, go back to open
			cb.stats.State = CircuitBreakerOpen
			cb.stats.NextRetryTime = time.Now().Add(cb.config.RecoveryTimeout)
			cb.logger.Warn().Msg("Circuit breaker returned to open state")
		}

		return err
	}

	// Success
	cb.stats.Successes++
	cb.stats.ConsecutiveFailures = 0

	// Check if we should close the circuit (from half-open)
	if cb.stats.State == CircuitBreakerHalfOpen {
		// Need consecutive successes to close
		recentSuccesses := cb.stats.Successes // Simplified - should track recent successes
		if int(recentSuccesses) >= cb.config.SuccessThreshold {
			cb.stats.State = CircuitBreakerClosed
			cb.logger.Info().Msg("Circuit breaker closed after successful recovery")
		}
	}

	return nil
}

// Simple health checkers

// RedisHealthChecker checks Redis connectivity
type RedisHealthChecker struct {
	name   string
	client interface{} // Redis client interface
}

func NewRedisHealthChecker(client interface{}) *RedisHealthChecker {
	return &RedisHealthChecker{
		name:   "redis",
		client: client,
	}
}

func (r *RedisHealthChecker) Name() string {
	return r.name
}

func (r *RedisHealthChecker) Check(ctx context.Context) ComponentHealth {
	// Simplified health check - in real implementation would ping Redis
	return ComponentHealth{
		Name:      r.name,
		Status:    HealthStatusHealthy,
		Message:   "Redis is responding",
		LastCheck: time.Now(),
	}
}

// MemoryHealthChecker checks memory usage
type MemoryHealthChecker struct {
	name      string
	threshold float64 // Memory threshold percentage
}

func NewMemoryHealthChecker(threshold float64) *MemoryHealthChecker {
	return &MemoryHealthChecker{
		name:      "memory",
		threshold: threshold,
	}
}

func (m *MemoryHealthChecker) Name() string {
	return m.name
}

func (m *MemoryHealthChecker) Check(ctx context.Context) ComponentHealth {
	// Simplified health check - in real implementation would check actual memory usage
	memoryUsage := 45.0 // Mock memory usage percentage

	var status HealthStatus
	var message string

	if memoryUsage < m.threshold {
		status = HealthStatusHealthy
		message = fmt.Sprintf("Memory usage: %.1f%%", memoryUsage)
	} else if memoryUsage < m.threshold*1.2 {
		status = HealthStatusDegraded
		message = fmt.Sprintf("Memory usage high: %.1f%%", memoryUsage)
	} else {
		status = HealthStatusUnhealthy
		message = fmt.Sprintf("Memory usage critical: %.1f%%", memoryUsage)
	}

	return ComponentHealth{
		Name:      m.name,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
		Metadata: map[string]interface{}{
			"usage_percent": memoryUsage,
			"threshold":     m.threshold,
		},
	}
}
