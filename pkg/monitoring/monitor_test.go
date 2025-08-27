package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestHealthStatus(t *testing.T) {
	statuses := []HealthStatus{
		HealthStatusHealthy,
		HealthStatusDegraded,
		HealthStatusUnhealthy,
		HealthStatusUnknown,
	}

	expectedStatuses := []string{
		"healthy",
		"degraded",
		"unhealthy",
		"unknown",
	}

	for i, status := range statuses {
		assert.Equal(t, expectedStatuses[i], string(status))
	}
}

func TestAlertLevel(t *testing.T) {
	levels := []AlertLevel{
		AlertLevelInfo,
		AlertLevelWarning,
		AlertLevelCritical,
	}

	expectedLevels := []string{
		"info",
		"warning",
		"critical",
	}

	for i, level := range levels {
		assert.Equal(t, expectedLevels[i], string(level))
	}
}

func TestCircuitBreakerState(t *testing.T) {
	states := []CircuitBreakerState{
		CircuitBreakerClosed,
		CircuitBreakerOpen,
		CircuitBreakerHalfOpen,
	}

	expectedStates := []string{
		"closed",
		"open",
		"half_open",
	}

	for i, state := range states {
		assert.Equal(t, expectedStates[i], string(state))
	}
}

func TestDefaultMonitorConfig(t *testing.T) {
	config := DefaultMonitorConfig()

	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	assert.True(t, config.AlertingEnabled)
	assert.True(t, config.MetricsEnabled)
	assert.Len(t, config.CircuitBreakers, 2)

	// Check Redis circuit breaker config
	redisConfig := config.CircuitBreakers[0]
	assert.Equal(t, "redis", redisConfig.Name)
	assert.Equal(t, 5, redisConfig.FailureThreshold)
	assert.Equal(t, 30*time.Second, redisConfig.RecoveryTimeout)
	assert.Equal(t, 3, redisConfig.SuccessThreshold)
	assert.Equal(t, 5*time.Second, redisConfig.Timeout)
	assert.Equal(t, 100, redisConfig.MaxConcurrentCalls)

	// Check Database circuit breaker config
	dbConfig := config.CircuitBreakers[1]
	assert.Equal(t, "database", dbConfig.Name)
	assert.Equal(t, 3, dbConfig.FailureThreshold)
	assert.Equal(t, 60*time.Second, dbConfig.RecoveryTimeout)
	assert.Equal(t, 2, dbConfig.SuccessThreshold)
	assert.Equal(t, 10*time.Second, dbConfig.Timeout)
	assert.Equal(t, 50, dbConfig.MaxConcurrentCalls)
}

func TestComponentHealth(t *testing.T) {
	health := ComponentHealth{
		Name:       "test-component",
		Status:     HealthStatusHealthy,
		Message:    "Component is working properly",
		LastCheck:  time.Now(),
		CheckCount: 10,
		Duration:   50 * time.Millisecond,
		Metadata: map[string]interface{}{
			"cpu_usage":    25.5,
			"memory_usage": 60.0,
		},
	}

	assert.Equal(t, "test-component", health.Name)
	assert.Equal(t, HealthStatusHealthy, health.Status)
	assert.Equal(t, "Component is working properly", health.Message)
	assert.Equal(t, int64(10), health.CheckCount)
	assert.Equal(t, 50*time.Millisecond, health.Duration)
	assert.Equal(t, 25.5, health.Metadata["cpu_usage"])
	assert.Equal(t, 60.0, health.Metadata["memory_usage"])
}

func TestSystemHealth(t *testing.T) {
	systemHealth := SystemHealth{
		Status:    HealthStatusHealthy,
		Message:   "All systems operational",
		Timestamp: time.Now(),
		Components: map[string]ComponentHealth{
			"redis": {
				Name:    "redis",
				Status:  HealthStatusHealthy,
				Message: "Redis is responding",
			},
			"memory": {
				Name:    "memory",
				Status:  HealthStatusHealthy,
				Message: "Memory usage: 45.0%",
			},
		},
		Uptime:  1 * time.Hour,
		Version: "2.0.0",
	}

	assert.Equal(t, HealthStatusHealthy, systemHealth.Status)
	assert.Equal(t, "All systems operational", systemHealth.Message)
	assert.Len(t, systemHealth.Components, 2)
	assert.Equal(t, 1*time.Hour, systemHealth.Uptime)
	assert.Equal(t, "2.0.0", systemHealth.Version)
	assert.Contains(t, systemHealth.Components, "redis")
	assert.Contains(t, systemHealth.Components, "memory")
}

func TestAlert(t *testing.T) {
	alert := Alert{
		ID:          "alert-123",
		Level:       AlertLevelCritical,
		Component:   "database",
		Title:       "Database connection failed",
		Description: "Unable to connect to database after 3 retries",
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"retry_count": 3,
			"error_code":  "CONNECTION_TIMEOUT",
		},
		Resolved: false,
	}

	assert.Equal(t, "alert-123", alert.ID)
	assert.Equal(t, AlertLevelCritical, alert.Level)
	assert.Equal(t, "database", alert.Component)
	assert.Equal(t, "Database connection failed", alert.Title)
	assert.False(t, alert.Resolved)
	assert.Nil(t, alert.ResolvedAt)
	assert.Equal(t, 3, alert.Metadata["retry_count"])
	assert.Equal(t, "CONNECTION_TIMEOUT", alert.Metadata["error_code"])
}

func TestCircuitBreakerConfig(t *testing.T) {
	config := &CircuitBreakerConfig{
		Name:               "test-service",
		FailureThreshold:   5,
		RecoveryTimeout:    30 * time.Second,
		SuccessThreshold:   3,
		Timeout:            10 * time.Second,
		MaxConcurrentCalls: 100,
	}

	assert.Equal(t, "test-service", config.Name)
	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 30*time.Second, config.RecoveryTimeout)
	assert.Equal(t, 3, config.SuccessThreshold)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 100, config.MaxConcurrentCalls)
}

func TestCircuitBreakerStats(t *testing.T) {
	stats := CircuitBreakerStats{
		State:               CircuitBreakerClosed,
		Requests:            100,
		Successes:           95,
		Failures:            5,
		ConsecutiveFailures: 0,
		LastFailureTime:     time.Now().Add(-1 * time.Hour),
		NextRetryTime:       time.Time{},
	}

	assert.Equal(t, CircuitBreakerClosed, stats.State)
	assert.Equal(t, int64(100), stats.Requests)
	assert.Equal(t, int64(95), stats.Successes)
	assert.Equal(t, int64(5), stats.Failures)
	assert.Equal(t, 0, stats.ConsecutiveFailures)
}

func TestNewCircuitBreaker(t *testing.T) {
	logger := zerolog.New(nil)
	config := &CircuitBreakerConfig{
		Name:               "test-cb",
		FailureThreshold:   3,
		RecoveryTimeout:    30 * time.Second,
		SuccessThreshold:   2,
		Timeout:            5 * time.Second,
		MaxConcurrentCalls: 50,
	}

	cb := NewCircuitBreaker(config, logger)

	assert.NotNil(t, cb)
	assert.Equal(t, config, cb.config)
	assert.Equal(t, CircuitBreakerClosed, cb.stats.State)
	assert.Equal(t, int64(0), cb.stats.Requests)
	assert.Equal(t, int64(0), cb.stats.Successes)
	assert.Equal(t, int64(0), cb.stats.Failures)
	assert.Equal(t, 0, cb.stats.ConsecutiveFailures)
}

func TestNewMonitor(t *testing.T) {
	logger := zerolog.New(nil)
	config := DefaultMonitorConfig()

	monitor := NewMonitor(config, logger)

	assert.NotNil(t, monitor)
	assert.Equal(t, config, monitor.config)
	assert.NotNil(t, monitor.checkers)
	assert.NotNil(t, monitor.alertHandlers)
	assert.NotNil(t, monitor.circuitBreakers)
	assert.Equal(t, HealthStatusUnknown, monitor.systemHealth.Status)
	assert.Equal(t, "2.0.0", monitor.systemHealth.Version)
	assert.Len(t, monitor.circuitBreakers, 2)
}

func TestRedisHealthChecker(t *testing.T) {
	checker := NewRedisHealthChecker(nil) // Mock client

	assert.Equal(t, "redis", checker.Name())

	ctx := context.Background()
	health := checker.Check(ctx)

	assert.Equal(t, "redis", health.Name)
	assert.Equal(t, HealthStatusHealthy, health.Status)
	assert.Equal(t, "Redis is responding", health.Message)
	assert.False(t, health.LastCheck.IsZero())
}

func TestMemoryHealthChecker(t *testing.T) {
	threshold := 80.0
	checker := NewMemoryHealthChecker(threshold)

	assert.Equal(t, "memory", checker.Name())

	ctx := context.Background()
	health := checker.Check(ctx)

	assert.Equal(t, "memory", health.Name)
	assert.Equal(t, HealthStatusHealthy, health.Status) // Mock usage is 45%, below 80%
	assert.Contains(t, health.Message, "Memory usage:")
	assert.False(t, health.LastCheck.IsZero())
	assert.Equal(t, 45.0, health.Metadata["usage_percent"])
	assert.Equal(t, threshold, health.Metadata["threshold"])
}

// Mock health checker for testing
type MockHealthChecker struct {
	name    string
	status  HealthStatus
	message string
}

func (m *MockHealthChecker) Name() string {
	return m.name
}

func (m *MockHealthChecker) Check(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Name:      m.name,
		Status:    m.status,
		Message:   m.message,
		LastCheck: time.Now(),
	}
}

func TestMonitorRegisterHealthChecker(t *testing.T) {
	logger := zerolog.New(nil)
	monitor := NewMonitor(nil, logger)

	checker := &MockHealthChecker{
		name:    "test-component",
		status:  HealthStatusHealthy,
		message: "Test component is healthy",
	}

	monitor.RegisterHealthChecker(checker)

	assert.Contains(t, monitor.checkers, "test-component")
	assert.Equal(t, checker, monitor.checkers["test-component"])
}

// Mock alert handler for testing
type MockAlertHandler struct {
	alerts []Alert
}

func (m *MockAlertHandler) HandleAlert(alert Alert) error {
	m.alerts = append(m.alerts, alert)
	return nil
}

func TestMonitorRegisterAlertHandler(t *testing.T) {
	logger := zerolog.New(nil)
	monitor := NewMonitor(nil, logger)

	handler := &MockAlertHandler{
		alerts: make([]Alert, 0),
	}

	monitor.RegisterAlertHandler(handler)

	assert.Len(t, monitor.alertHandlers, 1)
}

func TestMonitorGetSystemHealth(t *testing.T) {
	logger := zerolog.New(nil)
	monitor := NewMonitor(nil, logger)

	health := monitor.GetSystemHealth()

	assert.Equal(t, HealthStatusUnknown, health.Status)
	assert.Equal(t, "2.0.0", health.Version)
	assert.False(t, health.Timestamp.IsZero())
	assert.Greater(t, health.Uptime, time.Duration(0))
}

func TestCircuitBreakerExecute(t *testing.T) {
	logger := zerolog.New(nil)
	config := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 2,
	}

	cb := NewCircuitBreaker(config, logger)

	// Test successful execution
	err := cb.Execute(func() error {
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), cb.stats.Requests)
	assert.Equal(t, int64(1), cb.stats.Successes)
	assert.Equal(t, int64(0), cb.stats.Failures)
	assert.Equal(t, CircuitBreakerClosed, cb.stats.State)
}

func TestHealthStatusEvaluation(t *testing.T) {
	tests := []struct {
		name            string
		healthyCount    int
		degradedCount   int
		unhealthyCount  int
		totalComponents int
		expectedStatus  HealthStatus
		expectedMessage string
	}{
		{
			name:            "all healthy",
			healthyCount:    3,
			degradedCount:   0,
			unhealthyCount:  0,
			totalComponents: 3,
			expectedStatus:  HealthStatusHealthy,
			expectedMessage: "All 3 components healthy",
		},
		{
			name:            "some degraded",
			healthyCount:    2,
			degradedCount:   1,
			unhealthyCount:  0,
			totalComponents: 3,
			expectedStatus:  HealthStatusDegraded,
			expectedMessage: "1/3 components degraded",
		},
		{
			name:            "some unhealthy",
			healthyCount:    1,
			degradedCount:   1,
			unhealthyCount:  1,
			totalComponents: 3,
			expectedStatus:  HealthStatusUnhealthy,
			expectedMessage: "1/3 components unhealthy",
		},
		{
			name:            "no components",
			healthyCount:    0,
			degradedCount:   0,
			unhealthyCount:  0,
			totalComponents: 0,
			expectedStatus:  HealthStatusUnknown,
			expectedMessage: "No components registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status HealthStatus
			var message string

			if tt.totalComponents == 0 {
				status = HealthStatusUnknown
				message = "No components registered"
			} else if tt.unhealthyCount > 0 {
				status = HealthStatusUnhealthy
				message = fmt.Sprintf("%d/%d components unhealthy", tt.unhealthyCount, tt.totalComponents)
			} else if tt.degradedCount > 0 {
				status = HealthStatusDegraded
				message = fmt.Sprintf("%d/%d components degraded", tt.degradedCount, tt.totalComponents)
			} else {
				status = HealthStatusHealthy
				message = fmt.Sprintf("All %d components healthy", tt.totalComponents)
			}

			assert.Equal(t, tt.expectedStatus, status)
			assert.Equal(t, tt.expectedMessage, message)
		})
	}
}
