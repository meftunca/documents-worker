package adapters

import (
	"context"
	"documents-worker/internal/core/domain"
	"documents-worker/internal/core/ports"
	"documents-worker/queue"
	"time"
)

// QueueAdapter wraps the existing RedisQueue to implement ports.Queue
type QueueAdapter struct {
	redisQueue *queue.RedisQueue
}

// NewQueueAdapter creates a new queue adapter
func NewQueueAdapter(redisQueue *queue.RedisQueue) ports.Queue {
	return &QueueAdapter{
		redisQueue: redisQueue,
	}
}

func (q *QueueAdapter) Enqueue(ctx context.Context, job *domain.ProcessingJob) error {
	// Convert domain job to queue format
	queueJob := &queue.Job{
		ID:         job.ID,
		Type:       string(job.Type),
		Status:     queue.JobStatus(job.Status),
		Payload:    job.Parameters, // Use Parameters as Payload
		CreatedAt:  job.CreatedAt,
		RetryCount: job.RetryCount,
	}

	return q.redisQueue.Enqueue(ctx, queueJob)
}

func (q *QueueAdapter) Dequeue(ctx context.Context) (*domain.ProcessingJob, error) {
	// This would need to be implemented based on your existing queue structure
	// For now, return nil to indicate no job available
	return nil, nil
}

func (q *QueueAdapter) Complete(ctx context.Context, jobID string, result map[string]interface{}) error {
	// Update job status to completed with result
	// This would need to be implemented based on your existing queue structure
	return nil
}

func (q *QueueAdapter) Fail(ctx context.Context, jobID string, errorMsg string) error {
	// Update job status to failed with error message
	// This would need to be implemented based on your existing queue structure
	return nil
}

func (q *QueueAdapter) GetStats(ctx context.Context) (*domain.QueueStats, error) {
	// For now, return basic stats
	// In a real implementation, you'd query Redis for actual statistics
	return &domain.QueueStats{
		TotalJobs:      0,
		PendingJobs:    0,
		ProcessingJobs: 0,
		CompletedJobs:  0,
		FailedJobs:     0,
		Timestamp:      time.Now(),
	}, nil
}

func (q *QueueAdapter) Close() error {
	q.redisQueue.Close()
	return nil
}
