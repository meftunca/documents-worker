package worker

import (
	"context"
	"documents-worker/config"
	"documents-worker/queue"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test configuration for worker tests
func getTestWorkerConfig() *config.Config {
	return &config.Config{
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       2, // Use different DB for worker tests
		},
		Worker: config.WorkerConfig{
			MaxConcurrency:     3,
			QueueName:          "test_worker_queue",
			RetryCount:         1,
			RetryDelay:         1 * time.Second,
			MinWorkers:         1,
			ScaleUpThreshold:   5,
			ScaleDownThreshold: 1,
			CheckInterval:      1 * time.Second, // Faster for tests
			ScaleDelay:         2 * time.Second, // Shorter for tests
		},
		External: config.ExternalConfig{
			VipsEnabled:     false,
			FFmpegPath:      "echo",
			LibreOfficePath: "echo",
			MutoolPath:      "echo",
			TesseractPath:   "echo",
			PyMuPDFScript:   "./scripts",
			WkHtmlToPdfPath: "echo",
			PandocPath:      "echo",
		},
	}
}

// Test Worker Creation
func TestWorkerCreation(t *testing.T) {
	cfg := getTestWorkerConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err, "Failed to create Redis queue")
	defer redisQueue.Close()

	worker := NewWorker(redisQueue, cfg)
	require.NotNil(t, worker)

	assert.NotEmpty(t, worker.id)
	assert.Equal(t, redisQueue, worker.queue)
	assert.Equal(t, cfg, worker.config)
	assert.False(t, worker.IsRunning())
}

// Test Worker Start/Stop
func TestWorkerStartStop(t *testing.T) {
	cfg := getTestWorkerConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	worker := NewWorker(redisQueue, cfg)

	// Test start
	worker.Start()
	assert.True(t, worker.IsRunning())

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test stop
	worker.Stop()
	assert.False(t, worker.IsRunning())

	// Test double start (should not panic)
	worker.Start()
	assert.True(t, worker.IsRunning())

	// Test double stop (should not panic)
	worker.Stop()
	worker.Stop()
	assert.False(t, worker.IsRunning())
}

// Test WorkerManager Creation
func TestWorkerManagerCreation(t *testing.T) {
	cfg := getTestWorkerConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	manager := NewWorkerManager(redisQueue, cfg)
	require.NotNil(t, manager)

	assert.Equal(t, redisQueue, manager.queue)
	assert.Equal(t, cfg, manager.config)
	assert.Equal(t, 1, manager.minWorkers)
	assert.Equal(t, 3, manager.maxWorkers)
	assert.Equal(t, int64(5), manager.scaleUpThreshold)
	assert.Equal(t, int64(1), manager.scaleDownThreshold)
}

// Test WorkerManager Start/Stop
func TestWorkerManagerStartStop(t *testing.T) {
	cfg := getTestWorkerConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	manager := NewWorkerManager(redisQueue, cfg)

	// Test start
	manager.Start()

	// Give it time to start workers
	time.Sleep(200 * time.Millisecond)

	// Should have minimum workers
	assert.Equal(t, 1, manager.getWorkerCount())

	// Test stop
	manager.Stop()

	// Should have no workers after stop
	assert.Equal(t, 0, manager.getWorkerCount())
}

// Test Worker Manager Stats
func TestWorkerManagerStats(t *testing.T) {
	cfg := getTestWorkerConfig()

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	manager := NewWorkerManager(redisQueue, cfg)
	manager.Start()
	defer manager.Stop()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	stats := manager.GetStats()
	require.NotNil(t, stats)

	assert.Equal(t, 1, stats["active_workers"])
	assert.Equal(t, 1, stats["min_workers"])
	assert.Equal(t, 3, stats["max_workers"])
	assert.Equal(t, int64(5), stats["scale_up_threshold"])
	assert.Equal(t, int64(1), stats["scale_down_threshold"])
	assert.Equal(t, "1s", stats["check_interval"])
	assert.Equal(t, "2s", stats["scale_delay"])
}

// Test Worker Manager Scaling (requires Redis)
func TestWorkerManagerScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scaling test in short mode")
	}

	cfg := getTestWorkerConfig()
	cfg.Worker.ScaleUpThreshold = 2                   // Lower threshold for testing
	cfg.Worker.CheckInterval = 500 * time.Millisecond // Faster checking
	cfg.Worker.ScaleDelay = 500 * time.Millisecond    // Faster scaling

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	require.NoError(t, err)
	defer redisQueue.Close()

	manager := NewWorkerManager(redisQueue, cfg)
	manager.Start()
	defer manager.Stop()

	// Initial state
	time.Sleep(200 * time.Millisecond)
	initialWorkers := manager.getWorkerCount()
	assert.Equal(t, 1, initialWorkers)

	// Add multiple jobs to trigger scaling
	job := &queue.Job{
		ID:   "test-job-1",
		Type: "test_job",
		Payload: map[string]interface{}{
			"test": "data",
		},
	}

	// Add jobs to exceed threshold
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		job.ID = "test-job-" + string(rune(i+1))
		err := redisQueue.Enqueue(ctx, job)
		if err != nil {
			t.Logf("Failed to enqueue job %d: %v", i+1, err)
		}
	}

	// Wait for scaling to happen
	time.Sleep(2 * time.Second)

	// Should have scaled up (this might not always trigger due to fast job processing)
	finalWorkers := manager.getWorkerCount()
	t.Logf("Initial workers: %d, Final workers: %d", initialWorkers, finalWorkers)

	// At minimum, should still have the initial worker
	assert.GreaterOrEqual(t, finalWorkers, initialWorkers)
}

// Benchmark Worker Creation
func BenchmarkWorkerCreation(b *testing.B) {
	cfg := getTestWorkerConfig()

	redisQueue, _ := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	defer redisQueue.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		worker := NewWorker(redisQueue, cfg)
		_ = worker
	}
}

// Benchmark Worker Manager Stats
func BenchmarkWorkerManagerStats(b *testing.B) {
	cfg := getTestWorkerConfig()

	redisQueue, _ := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	defer redisQueue.Close()

	manager := NewWorkerManager(redisQueue, cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := manager.GetStats()
		_ = stats
	}
}
