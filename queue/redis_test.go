package queue

import (
	"context"
	"documents-worker/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test configuration for queue tests
func getTestQueueConfig() (*config.RedisConfig, *config.WorkerConfig) {
	redisConfig := &config.RedisConfig{
		Host:     "localhost",
		Port:     "6379",
		Password: "",
		DB:       4, // Use different DB for queue tests
	}

	workerConfig := &config.WorkerConfig{
		QueueName:  "test_queue_tests",
		RetryCount: 2,
		RetryDelay: 1 * time.Second,
	}

	return redisConfig, workerConfig
}

// Test Redis Queue Creation
func TestRedisQueueCreation(t *testing.T) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, err := NewRedisQueue(redisConfig, workerConfig)
	require.NoError(t, err)
	require.NotNil(t, queue)

	defer queue.Close()

	assert.NotNil(t, queue.client)
	assert.Equal(t, workerConfig, queue.config)
}

// Test Job Enqueue and Dequeue
func TestJobEnqueueDequeue(t *testing.T) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, err := NewRedisQueue(redisConfig, workerConfig)
	require.NoError(t, err)
	defer queue.Close()

	ctx := context.Background()

	// Clean up any existing test data
	queue.client.FlushDB(ctx)

	// Create test job
	job := &Job{
		ID:   "test-job-1",
		Type: "test_type",
		Payload: map[string]interface{}{
			"test_data": "test_value",
		},
	}

	// Enqueue job
	err = queue.Enqueue(ctx, job)
	require.NoError(t, err)

	// Check job was stored
	retrievedJob, err := queue.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, retrievedJob.ID)
	assert.Equal(t, job.Type, retrievedJob.Type)
	assert.Equal(t, StatusPending, retrievedJob.Status)

	// Dequeue job
	dequeuedJob, err := queue.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, job.ID, dequeuedJob.ID)
	assert.Equal(t, StatusProcessing, dequeuedJob.Status)
}

// Test Job Completion
func TestJobCompletion(t *testing.T) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, err := NewRedisQueue(redisConfig, workerConfig)
	require.NoError(t, err)
	defer queue.Close()

	ctx := context.Background()

	// Clean up any existing test data
	queue.client.FlushDB(ctx)

	// Create and enqueue job
	job := &Job{
		ID:   "test-completion-job",
		Type: "test_type",
		Payload: map[string]interface{}{
			"test": "data",
		},
	}

	err = queue.Enqueue(ctx, job)
	require.NoError(t, err)

	// Dequeue job
	dequeuedJob, err := queue.Dequeue(ctx)
	require.NoError(t, err)

	// Complete job
	result := map[string]interface{}{
		"output": "test_output",
		"status": "success",
	}

	err = queue.CompleteJob(ctx, dequeuedJob.ID, result)
	require.NoError(t, err)

	// Check job status
	completedJob, err := queue.GetJob(ctx, dequeuedJob.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, completedJob.Status)
	assert.Equal(t, result, completedJob.Result)
	assert.NotNil(t, completedJob.CompletedAt)
}

// Test Job Failure and Retry
func TestJobFailureAndRetry(t *testing.T) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, err := NewRedisQueue(redisConfig, workerConfig)
	require.NoError(t, err)
	defer queue.Close()

	ctx := context.Background()

	// Clean up any existing test data
	queue.client.FlushDB(ctx)

	// Create and enqueue job
	job := &Job{
		ID:   "test-failure-job",
		Type: "test_type",
		Payload: map[string]interface{}{
			"test": "data",
		},
	}

	err = queue.Enqueue(ctx, job)
	require.NoError(t, err)

	// Dequeue job
	dequeuedJob, err := queue.Dequeue(ctx)
	require.NoError(t, err)

	// Fail job (should retry)
	err = queue.FailJob(ctx, dequeuedJob.ID, "Test error")
	require.NoError(t, err)

	// Check job was marked for retry
	failedJob, err := queue.GetJob(ctx, dequeuedJob.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, failedJob.RetryCount)
	assert.Equal(t, "Test error", failedJob.Error)

	// Fail again (should still retry)
	err = queue.FailJob(ctx, failedJob.ID, "Test error 2")
	require.NoError(t, err)

	// Check final failure
	finalJob, err := queue.GetJob(ctx, failedJob.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, finalJob.Status)
	assert.Equal(t, 2, finalJob.RetryCount)
}

// Test Queue Stats
func TestQueueStats(t *testing.T) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, err := NewRedisQueue(redisConfig, workerConfig)
	require.NoError(t, err)
	defer queue.Close()

	ctx := context.Background()

	// Clean up any existing test data
	queue.client.FlushDB(ctx)

	// Get initial stats
	stats, err := queue.GetQueueStats(ctx)
	require.NoError(t, err)

	initialPending := stats["pending"]

	// Add a job
	job := &Job{
		ID:   "test-stats-job",
		Type: "test_type",
		Payload: map[string]interface{}{
			"test": "data",
		},
	}

	err = queue.Enqueue(ctx, job)
	require.NoError(t, err)

	// Check stats updated
	stats, err = queue.GetQueueStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, initialPending+1, stats["pending"])
}

// Test Dequeue Timeout
func TestDequeueTimeout(t *testing.T) {
	redisConfig, workerConfig := getTestQueueConfig()

	// Use a unique queue name for this test to avoid interference
	workerConfig.QueueName = "test-timeout-queue-" + string(rune(time.Now().UnixNano()))

	queue, err := NewRedisQueue(redisConfig, workerConfig)
	if err != nil {
		t.Skip("Cannot create Redis queue (Redis likely not available):", err)
	}
	defer queue.Close()

	ctx := context.Background()

	// Clean up any existing test data
	queue.client.FlushDB(ctx)

	// Test context cancellation behavior with empty queue
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	job, err := queue.Dequeue(ctx)
	duration := time.Since(start)

	// Should return nil job since queue is empty
	assert.Nil(t, job, "Expected nil job when dequeueing from empty queue")

	// The operation should complete near our context timeout (100ms) but allow up to 6s for Redis timeout
	assert.True(t, duration < 6*time.Second,
		"Dequeue should not block indefinitely, got %v", duration)

	t.Logf("Dequeue completed in %v with error: %v", duration, err)
}

// Test Context Cancellation
func TestContextCancellation(t *testing.T) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, err := NewRedisQueue(redisConfig, workerConfig)
	require.NoError(t, err)
	defer queue.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Clean up any existing test data
	queue.client.FlushDB(ctx)

	// Cancel context immediately
	cancel()

	// Operations should fail with context cancellation
	job := &Job{
		ID:   "test-cancel-job",
		Type: "test_type",
		Payload: map[string]interface{}{
			"test": "data",
		},
	}

	err = queue.Enqueue(ctx, job)
	assert.Error(t, err)
}

// Benchmark Job Enqueue
func BenchmarkJobEnqueue(b *testing.B) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, _ := NewRedisQueue(redisConfig, workerConfig)
	defer queue.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &Job{
			ID:   "bench-job-" + string(rune(i)),
			Type: "bench_type",
			Payload: map[string]interface{}{
				"data": i,
			},
		}
		queue.Enqueue(ctx, job)
	}
}

// Benchmark Queue Stats
func BenchmarkQueueStats(b *testing.B) {
	redisConfig, workerConfig := getTestQueueConfig()

	queue, _ := NewRedisQueue(redisConfig, workerConfig)
	defer queue.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats, _ := queue.GetQueueStats(ctx)
		_ = stats
	}
}
