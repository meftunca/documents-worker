package health

import (
	"documents-worker/config"
	"documents-worker/queue"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test configuration for health tests
func getTestHealthConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Environment: "test",
		},
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       3, // Use different DB for health tests
		},
		Worker: config.WorkerConfig{
			QueueName: "test_health_queue",
		},
		External: config.ExternalConfig{
			VipsEnabled:     false,
			FFmpegPath:      "echo", // Mock command that exists
			LibreOfficePath: "echo",
			MutoolPath:      "echo",
			TesseractPath:   "echo",
		},
	}
}

// Test HealthChecker Creation
func TestHealthCheckerCreation(t *testing.T) {
	cfg := getTestHealthConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)
	require.NotNil(t, healthChecker)

	assert.Equal(t, cfg, healthChecker.config)
	assert.Equal(t, redisQueue, healthChecker.queue)
	assert.NotNil(t, healthChecker.cachedServices)
	assert.Equal(t, 5*time.Minute, healthChecker.serviceCheckTTL)
}

// Test Health Status
func TestHealthStatus(t *testing.T) {
	cfg := getTestHealthConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)

	status := healthChecker.GetHealthStatus()

	assert.NotEmpty(t, status.Status)
	assert.Equal(t, "1.0.0", status.Version)
	assert.NotZero(t, status.Timestamp)
	assert.NotEmpty(t, status.Uptime)
	assert.NotNil(t, status.Services)
	assert.NotNil(t, status.Queue)
	assert.Equal(t, "test", status.System.Environment)
}

// Test Service Caching
func TestServiceCaching(t *testing.T) {
	cfg := getTestHealthConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)
	healthChecker.serviceCheckTTL = 100 * time.Millisecond // Short TTL for testing

	// First call should populate cache
	status1 := healthChecker.GetHealthStatus()
	assert.NotEmpty(t, status1.Services)

	// Second call should use cache
	status2 := healthChecker.GetHealthStatus()
	assert.Equal(t, len(status1.Services), len(status2.Services))

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call should refresh cache
	status3 := healthChecker.GetHealthStatus()
	assert.NotEmpty(t, status3.Services)
}

// Test Health Handler
func TestHealthHandler(t *testing.T) {
	cfg := getTestHealthConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)

	app := fiber.New()
	app.Get("/health", healthChecker.HealthHandler)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	// Should return 200 or 503 depending on Redis connection
	assert.True(t, resp.StatusCode == fiber.StatusOK || resp.StatusCode == fiber.StatusServiceUnavailable)
}

// Test Fast Health Handler
func TestFastHealthHandler(t *testing.T) {
	cfg := getTestHealthConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)

	app := fiber.New()
	app.Get("/health/fast", healthChecker.FastHealthHandler)

	req := httptest.NewRequest("GET", "/health/fast", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	// Should return 200 or 503 depending on Redis connection
	assert.True(t, resp.StatusCode == fiber.StatusOK || resp.StatusCode == fiber.StatusServiceUnavailable)
}

// Test Liveness Handler
func TestLivenessHandler(t *testing.T) {
	cfg := getTestHealthConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)

	app := fiber.New()
	app.Get("/health/liveness", healthChecker.LivenessHandler)

	req := httptest.NewRequest("GET", "/health/liveness", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	// Liveness should always return 200
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test Readiness Handler
func TestReadinessHandler(t *testing.T) {
	cfg := getTestHealthConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)

	app := fiber.New()
	app.Get("/health/readiness", healthChecker.ReadinessHandler)

	req := httptest.NewRequest("GET", "/health/readiness", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	// Should return 200 or 503 depending on Redis connection
	assert.True(t, resp.StatusCode == fiber.StatusOK || resp.StatusCode == fiber.StatusServiceUnavailable)
}

// Test with nil queue
func TestHealthCheckerWithNilQueue(t *testing.T) {
	cfg := getTestHealthConfig()

	healthChecker := NewHealthChecker(cfg, nil)

	status := healthChecker.GetHealthStatus()

	assert.Equal(t, "unhealthy", status.Status)
	assert.False(t, status.Queue.Connected)
	assert.Equal(t, "Queue not initialized", status.Queue.Error)
}

// Benchmark Health Status
func BenchmarkHealthStatus(b *testing.B) {
	cfg := getTestHealthConfig()

	redisQueue, _ := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		status := healthChecker.GetHealthStatus()
		_ = status
	}
}

// Benchmark Fast Health Handler
func BenchmarkFastHealthHandler(b *testing.B) {
	cfg := getTestHealthConfig()

	redisQueue, _ := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	defer redisQueue.Close()

	healthChecker := NewHealthChecker(cfg, redisQueue)

	app := fiber.New()
	app.Get("/health/fast", healthChecker.FastHealthHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/health/fast", nil)
		resp, _ := app.Test(req, -1)
		resp.Body.Close()
	}
}
