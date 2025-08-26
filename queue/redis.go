package queue

import (
	"context"
	"documents-worker/config"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client *redis.Client
	config *config.WorkerConfig
}

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type Job struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Status      JobStatus              `json:"status"`
	Payload     map[string]interface{} `json:"payload"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
}

func NewRedisQueue(redisConfig *config.RedisConfig, workerConfig *config.WorkerConfig) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisConfig.Host, redisConfig.Port),
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &RedisQueue{
		client: client,
		config: workerConfig,
	}, nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, job *Job) error {
	job.Status = StatusPending
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()
	job.MaxRetries = q.config.RetryCount

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Add to processing queue
	if err := q.client.LPush(ctx, q.config.QueueName, jobData).Err(); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Store job details with expiration (24 hours)
	jobKey := fmt.Sprintf("job:%s", job.ID)
	if err := q.client.Set(ctx, jobKey, jobData, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to store job details: %w", err)
	}

	return nil
}

func (q *RedisQueue) Dequeue(ctx context.Context) (*Job, error) {
	// Use a timeout for BRPOP to allow graceful shutdown
	result, err := q.client.BRPop(ctx, 5*time.Second, q.config.QueueName).Result()
	if err != nil {
		// Check if it's a timeout or context cancellation
		if err == redis.Nil || ctx.Err() != nil {
			return nil, err
		}
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("invalid queue result")
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	// Update status to processing
	job.Status = StatusProcessing
	job.UpdatedAt = time.Now()

	if err := q.updateJob(ctx, &job); err != nil {
		return nil, fmt.Errorf("failed to update job status: %w", err)
	}

	return &job, nil
}

func (q *RedisQueue) CompleteJob(ctx context.Context, jobID string, result map[string]interface{}) error {
	job, err := q.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	now := time.Now()
	job.Status = StatusCompleted
	job.Result = result
	job.UpdatedAt = now
	job.CompletedAt = &now

	return q.updateJob(ctx, job)
}

func (q *RedisQueue) FailJob(ctx context.Context, jobID string, errorMsg string) error {
	job, err := q.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	job.RetryCount++
	job.Error = errorMsg
	job.UpdatedAt = time.Now()

	// If max retries reached, mark as failed
	if job.RetryCount >= job.MaxRetries {
		job.Status = StatusFailed
		return q.updateJob(ctx, job)
	}

	// Otherwise, retry after delay
	job.Status = StatusPending
	if err := q.updateJob(ctx, job); err != nil {
		return err
	}

	// Re-enqueue with delay
	go func() {
		time.Sleep(q.config.RetryDelay)
		q.Enqueue(context.Background(), job)
	}()

	return nil
}

func (q *RedisQueue) GetJob(ctx context.Context, jobID string) (*Job, error) {
	jobKey := fmt.Sprintf("job:%s", jobID)
	jobData, err := q.client.Get(ctx, jobKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

func (q *RedisQueue) GetQueueStats(ctx context.Context) (map[string]int64, error) {
	queueLength, err := q.client.LLen(ctx, q.config.QueueName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue length: %w", err)
	}

	return map[string]int64{
		"pending": queueLength,
	}, nil
}

func (q *RedisQueue) updateJob(ctx context.Context, job *Job) error {
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	jobKey := fmt.Sprintf("job:%s", job.ID)
	if err := q.client.Set(ctx, jobKey, jobData, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return nil
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}
