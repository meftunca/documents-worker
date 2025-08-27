package worker

import (
	"context"
	"documents-worker/config"
	"documents-worker/media"
	"documents-worker/pkg/memory"
	"documents-worker/queue"
	"documents-worker/textextractor"
	"documents-worker/types"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Worker struct {
	id            string
	queue         *queue.RedisQueue
	config        *config.Config
	textExtractor *textextractor.TextExtractor
	memoryManager *memory.Manager
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	isRunning     bool
	runningMutex  sync.RWMutex
}

type ProcessingJob struct {
	ID           string                 `json:"id"`
	InputPath    string                 `json:"input_path"`
	MediaKind    types.MediaKind        `json:"media_kind"`
	SearchParams types.MediaSearch      `json:"search_params"`
	Format       *string                `json:"format,omitempty"`
	VipsEnabled  bool                   `json:"vips_enabled"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

func NewWorker(queue *queue.RedisQueue, config *config.Config) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	textExtractor := textextractor.NewTextExtractor(&config.External)
	memManager := memory.NewManager(nil)

	return &Worker{
		id:            uuid.New().String(),
		queue:         queue,
		config:        config,
		textExtractor: textExtractor,
		memoryManager: memManager,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (w *Worker) Start() {
	w.runningMutex.Lock()
	defer w.runningMutex.Unlock()

	if w.isRunning {
		return
	}

	log.Printf("Worker %s starting", w.id)
	w.isRunning = true

	// Start single worker routine for this worker instance
	w.wg.Add(1)
	go w.workerRoutine()
}

func (w *Worker) Stop() {
	w.runningMutex.Lock()
	if !w.isRunning {
		w.runningMutex.Unlock()
		return
	}
	w.isRunning = false
	w.runningMutex.Unlock()

	log.Printf("Worker %s stopping...", w.id)
	w.cancel()
	w.wg.Wait()
	log.Printf("Worker %s stopped", w.id)
}

func (w *Worker) IsRunning() bool {
	w.runningMutex.RLock()
	defer w.runningMutex.RUnlock()
	return w.isRunning
}

func (w *Worker) workerRoutine() {
	defer w.wg.Done()

	log.Printf("Worker %s routine started", w.id)

	for {
		// Check if context is cancelled first
		select {
		case <-w.ctx.Done():
			log.Printf("Worker %s routine stopping", w.id)
			return
		default:
		}

		// Try to dequeue job with context
		job, err := w.queue.Dequeue(w.ctx)

		if err != nil {
			if w.ctx.Err() != nil {
				log.Printf("Worker %s routine stopping due to context cancellation", w.id)
				return // Context cancelled
			}
			// Redis timeout or no jobs available
			if err.Error() == "redis: nil" {
				continue // No jobs available, continue polling
			}
			log.Printf("Worker %s: Failed to dequeue job: %v", w.id, err)
			time.Sleep(1 * time.Second)
			continue
		}

		w.processJob(job)
	}
}

func (w *Worker) processJob(job *queue.Job) {
	log.Printf("Worker %s: Processing job %s (type: %s)", w.id, job.ID, job.Type)

	startTime := time.Now()

	switch job.Type {
	case "media_processing":
		w.processMediaJob(job)
	case "ocr_processing":
		w.processOCRJob(job)
	case "text_extraction":
		w.processTextExtractionJob(job)
	case "export_processing":
		w.processExportJob(job)
	default:
		err := fmt.Sprintf("Unknown job type: %s", job.Type)
		w.queue.FailJob(context.Background(), job.ID, err)
		log.Printf("Worker %s: %s", w.id, err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("Worker %s: Job %s completed in %v", w.id, job.ID, duration)
}

func (w *Worker) processMediaJob(job *queue.Job) {
	// Parse job payload
	var processingJob ProcessingJob
	payloadBytes, err := json.Marshal(job.Payload)
	if err != nil {
		w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Failed to marshal job payload: %v", err))
		return
	}

	if err := json.Unmarshal(payloadBytes, &processingJob); err != nil {
		w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Failed to unmarshal job payload: %v", err))
		return
	}

	// Create media converter
	mediaConverter := &types.MediaConverter{
		Kind:        processingJob.MediaKind,
		Search:      processingJob.SearchParams,
		Format:      processingJob.Format,
		VipsEnabled: processingJob.VipsEnabled,
	}

	// Create processor
	processor, err := media.NewProcessor(mediaConverter)
	if err != nil {
		w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Failed to create processor: %v", err))
		return
	}

	// Process file
	outputFile, err := processor.Process(processingJob.InputPath)
	if err != nil {
		w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Failed to process file: %v", err))
		return
	}
	defer outputFile.Close()
	defer os.Remove(outputFile.Name())

	// Prepare result
	result := map[string]interface{}{
		"output_path":  outputFile.Name(),
		"processed_at": time.Now(),
		"input_path":   processingJob.InputPath,
		"media_kind":   processingJob.MediaKind,
	}

	// Add metadata if available
	if processingJob.Metadata != nil {
		result["metadata"] = processingJob.Metadata
	}

	// Complete job
	if err := w.queue.CompleteJob(context.Background(), job.ID, result); err != nil {
		log.Printf("Failed to complete job %s: %v", job.ID, err)
	}
}

func (w *Worker) processOCRJob(job *queue.Job) {
	// TODO: Implement OCR processing
	// This will be implemented when we add OCR functionality
	result := map[string]interface{}{
		"status":  "not_implemented",
		"message": "OCR processing will be implemented in the next phase",
	}

	w.queue.CompleteJob(context.Background(), job.ID, result)
}

func (w *Worker) processTextExtractionJob(job *queue.Job) {
	// Parse job payload
	var textExtractionJob struct {
		ID        string                 `json:"id"`
		InputPath string                 `json:"input_path"`
		JobType   string                 `json:"job_type"` // "full", "pages", "range"
		StartPage *int                   `json:"start_page,omitempty"`
		EndPage   *int                   `json:"end_page,omitempty"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	}

	payloadBytes, err := json.Marshal(job.Payload)
	if err != nil {
		w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Failed to marshal job payload: %v", err))
		return
	}

	if err := json.Unmarshal(payloadBytes, &textExtractionJob); err != nil {
		w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Failed to unmarshal job payload: %v", err))
		return
	}

	var result map[string]interface{}

	switch textExtractionJob.JobType {
	case "full":
		extractionResult, err := w.textExtractor.ExtractFromFile(textExtractionJob.InputPath)
		if err != nil {
			w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Text extraction failed: %v", err))
			return
		}
		result = map[string]interface{}{
			"extraction_result": extractionResult,
			"job_type":          "full",
		}

	case "pages":
		extractionResults, err := w.textExtractor.BatchExtractPDFPages(textExtractionJob.InputPath)
		if err != nil {
			w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("PDF pages extraction failed: %v", err))
			return
		}
		result = map[string]interface{}{
			"extraction_results": extractionResults,
			"job_type":           "pages",
			"total_pages":        len(extractionResults),
		}

	case "range":
		if textExtractionJob.StartPage == nil || textExtractionJob.EndPage == nil {
			w.queue.FailJob(context.Background(), job.ID, "Range extraction requires start_page and end_page")
			return
		}
		extractionResult, err := w.textExtractor.ExtractByPages(
			textExtractionJob.InputPath,
			*textExtractionJob.StartPage,
			*textExtractionJob.EndPage,
		)
		if err != nil {
			w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("PDF range extraction failed: %v", err))
			return
		}
		result = map[string]interface{}{
			"extraction_result": extractionResult,
			"job_type":          "range",
			"start_page":        *textExtractionJob.StartPage,
			"end_page":          *textExtractionJob.EndPage,
		}

	default:
		w.queue.FailJob(context.Background(), job.ID, fmt.Sprintf("Unknown text extraction job type: %s", textExtractionJob.JobType))
		return
	}

	// Add common metadata
	result["processed_at"] = time.Now()
	result["input_path"] = textExtractionJob.InputPath

	if textExtractionJob.Metadata != nil {
		result["metadata"] = textExtractionJob.Metadata
	}

	// Complete job
	if err := w.queue.CompleteJob(context.Background(), job.ID, result); err != nil {
		log.Printf("Failed to complete text extraction job %s: %v", job.ID, err)
	}
}

func (w *Worker) processExportJob(job *queue.Job) {
	// TODO: Implement export processing
	// This will be implemented when we add export functionality
	result := map[string]interface{}{
		"status":  "not_implemented",
		"message": "Export processing will be implemented in the next phase",
	}

	w.queue.CompleteJob(context.Background(), job.ID, result)
}

// SubmitMediaJob creates and submits a media processing job to the queue
func SubmitMediaJob(q *queue.RedisQueue, inputPath string, mediaKind types.MediaKind,
	searchParams types.MediaSearch, format *string, vipsEnabled bool,
	metadata map[string]interface{}) (*queue.Job, error) {

	job := &queue.Job{
		ID:   uuid.New().String(),
		Type: "media_processing",
		Payload: map[string]interface{}{
			"id":            uuid.New().String(),
			"input_path":    inputPath,
			"media_kind":    mediaKind,
			"search_params": searchParams,
			"format":        format,
			"vips_enabled":  vipsEnabled,
			"metadata":      metadata,
		},
	}

	if err := q.Enqueue(context.Background(), job); err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	return job, nil
}

// SubmitTextExtractionJob creates and submits a text extraction job to the queue
func SubmitTextExtractionJob(q *queue.RedisQueue, inputPath string, jobType string,
	startPage, endPage *int, metadata map[string]interface{}) (*queue.Job, error) {

	payload := map[string]interface{}{
		"id":         uuid.New().String(),
		"input_path": inputPath,
		"job_type":   jobType,
		"metadata":   metadata,
	}

	if startPage != nil {
		payload["start_page"] = *startPage
	}
	if endPage != nil {
		payload["end_page"] = *endPage
	}

	job := &queue.Job{
		ID:      uuid.New().String(),
		Type:    "text_extraction",
		Payload: payload,
	}

	if err := q.Enqueue(context.Background(), job); err != nil {
		return nil, fmt.Errorf("failed to submit text extraction job: %w", err)
	}

	return job, nil
}
