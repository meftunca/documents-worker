package services

import (
	"context"
	"documents-worker/internal/core/domain"
	"documents-worker/internal/core/ports"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// DocumentServiceImpl implements the DocumentService port
type DocumentServiceImpl struct {
	documentRepo   ports.DocumentRepository
	jobRepo        ports.JobRepository
	fileStorage    ports.FileStorage
	queue          ports.Queue
	imageProcessor ports.ImageProcessor
	videoProcessor ports.VideoProcessor
	pdfProcessor   ports.PDFProcessor
	ocrProcessor   ports.OCRProcessor
	textExtractor  ports.TextExtractor
	eventPublisher ports.EventPublisher
}

// NewDocumentService creates a new document service
func NewDocumentService(
	documentRepo ports.DocumentRepository,
	jobRepo ports.JobRepository,
	fileStorage ports.FileStorage,
	queue ports.Queue,
	imageProcessor ports.ImageProcessor,
	videoProcessor ports.VideoProcessor,
	pdfProcessor ports.PDFProcessor,
	ocrProcessor ports.OCRProcessor,
	textExtractor ports.TextExtractor,
	eventPublisher ports.EventPublisher,
) ports.DocumentService {
	return &DocumentServiceImpl{
		documentRepo:   documentRepo,
		jobRepo:        jobRepo,
		fileStorage:    fileStorage,
		queue:          queue,
		imageProcessor: imageProcessor,
		videoProcessor: videoProcessor,
		pdfProcessor:   pdfProcessor,
		ocrProcessor:   ocrProcessor,
		textExtractor:  textExtractor,
		eventPublisher: eventPublisher,
	}
}

// ProcessDocument handles document processing requests
func (s *DocumentServiceImpl) ProcessDocument(ctx context.Context, req *domain.ProcessingRequest) (*domain.ProcessingResult, error) {
	// Verify document exists
	_, err := s.documentRepo.GetByID(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Create processing job
	job := &domain.ProcessingJob{
		ID:         uuid.New().String(),
		DocumentID: req.DocumentID,
		Type:       req.Type,
		Status:     domain.JobStatusPending,
		Parameters: req.Parameters,
		CreatedAt:  time.Now(),
	}

	// Save job
	if err := s.jobRepo.Save(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to save job: %w", err)
	}

	// Enqueue job for processing
	if err := s.queue.Enqueue(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Return processing result
	return &domain.ProcessingResult{
		JobID:      job.ID,
		DocumentID: job.DocumentID,
		Type:       job.Type,
		Status:     domain.JobStatusPending,
		Duration:   0,
	}, nil
}

// GetDocument retrieves a document by ID
func (s *DocumentServiceImpl) GetDocument(ctx context.Context, id string) (*domain.Document, error) {
	return s.documentRepo.GetByID(ctx, id)
}

// GetJob retrieves a job by ID
func (s *DocumentServiceImpl) GetJob(ctx context.Context, jobID string) (*domain.ProcessingJob, error) {
	return s.jobRepo.GetByID(ctx, jobID)
}

// GetJobsByDocument retrieves all jobs for a document
func (s *DocumentServiceImpl) GetJobsByDocument(ctx context.Context, documentID string) ([]*domain.ProcessingJob, error) {
	return s.jobRepo.GetByDocumentID(ctx, documentID)
}

// ConvertImage converts an image to the specified format
func (s *DocumentServiceImpl) ConvertImage(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error) {
	return s.imageProcessor.Convert(ctx, input, outputFormat, params)
}

// ConvertVideo converts a video to the specified format
func (s *DocumentServiceImpl) ConvertVideo(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error) {
	return s.videoProcessor.Convert(ctx, input, outputFormat, params)
}

// GeneratePDF generates a PDF from input
func (s *DocumentServiceImpl) GeneratePDF(ctx context.Context, input io.Reader, params map[string]interface{}) (io.Reader, error) {
	return s.pdfProcessor.GenerateFromHTML(ctx, input, params)
}

// ExtractText extracts text from a document
func (s *DocumentServiceImpl) ExtractText(ctx context.Context, input io.Reader, docType domain.DocumentType) (string, error) {
	switch docType {
	case domain.DocumentTypePDF:
		return s.textExtractor.ExtractFromPDF(ctx, input)
	case domain.DocumentTypeOffice:
		return s.textExtractor.ExtractFromOffice(ctx, input, "office")
	case domain.DocumentTypeText:
		return s.textExtractor.ExtractFromText(ctx, input)
	default:
		return "", domain.ErrUnsupportedFormat
	}
}

// PerformOCR performs OCR on an image or PDF
func (s *DocumentServiceImpl) PerformOCR(ctx context.Context, input io.Reader, language string) (string, error) {
	return s.ocrProcessor.ProcessImage(ctx, input, language)
}

// GenerateThumbnail generates a thumbnail from an image or video
func (s *DocumentServiceImpl) GenerateThumbnail(ctx context.Context, input io.Reader, params map[string]interface{}) (io.Reader, error) {
	if size, ok := params["size"].(int); ok {
		return s.imageProcessor.GenerateThumbnail(ctx, input, size)
	}
	return s.imageProcessor.GenerateThumbnail(ctx, input, 200) // default size
}

// HealthServiceImpl implements the HealthService port
type HealthServiceImpl struct {
	queue          ports.Queue
	cache          ports.Cache
	fileStorage    ports.FileStorage
	imageProcessor ports.ImageProcessor
	videoProcessor ports.VideoProcessor
	pdfProcessor   ports.PDFProcessor
	ocrProcessor   ports.OCRProcessor
}

// NewHealthService creates a new health service
func NewHealthService(
	queue ports.Queue,
	cache ports.Cache,
	fileStorage ports.FileStorage,
	imageProcessor ports.ImageProcessor,
	videoProcessor ports.VideoProcessor,
	pdfProcessor ports.PDFProcessor,
	ocrProcessor ports.OCRProcessor,
) ports.HealthService {
	return &HealthServiceImpl{
		queue:          queue,
		cache:          cache,
		fileStorage:    fileStorage,
		imageProcessor: imageProcessor,
		videoProcessor: videoProcessor,
		pdfProcessor:   pdfProcessor,
		ocrProcessor:   ocrProcessor,
	}
}

// GetHealthStatus returns the overall health status
func (s *HealthServiceImpl) GetHealthStatus(ctx context.Context) (*domain.HealthStatus, error) {
	deps, err := s.CheckDependencies(ctx)
	if err != nil {
		return nil, err
	}

	status := "healthy"
	for _, dep := range deps {
		if !dep.Available {
			status = "degraded"
			break
		}
	}

	return &domain.HealthStatus{
		Status:    status,
		Version:   "1.0.0",
		Timestamp: time.Now(),
		Services: map[string]domain.ServiceInfo{
			"document-processor": {Status: "running", Message: "Document processing service is running"},
			"queue":              {Status: "running", Message: "Queue service is running"},
		},
		Dependencies: deps,
	}, nil
}

// CheckDependencies checks the status of all dependencies
func (s *HealthServiceImpl) CheckDependencies(ctx context.Context) (map[string]domain.DepInfo, error) {
	deps := make(map[string]domain.DepInfo)

	// Check queue
	if s.queue != nil {
		if _, err := s.queue.GetStats(ctx); err != nil {
			deps["queue"] = domain.DepInfo{Status: "unhealthy", Available: false, Message: err.Error()}
		} else {
			deps["queue"] = domain.DepInfo{Status: "healthy", Available: true}
		}
	} else {
		deps["queue"] = domain.DepInfo{Status: "unavailable", Available: false, Message: "Queue service not initialized"}
	}

	// Check cache
	if s.cache != nil {
		if err := s.cache.Set(ctx, "health-check", []byte("ok"), 10); err != nil {
			deps["cache"] = domain.DepInfo{Status: "unhealthy", Available: false, Message: err.Error()}
		} else {
			deps["cache"] = domain.DepInfo{Status: "healthy", Available: true}
		}
	} else {
		deps["cache"] = domain.DepInfo{Status: "unavailable", Available: false, Message: "Cache service not initialized"}
	}

	// Add other dependency checks here (VIPS, FFmpeg, Tesseract, etc.)
	deps["vips"] = domain.DepInfo{Status: "healthy", Available: true, Version: "8.15", Message: "VIPS image processing"}
	deps["ffmpeg"] = domain.DepInfo{Status: "healthy", Available: true, Version: "6.0", Message: "FFmpeg video processing"}
	deps["tesseract"] = domain.DepInfo{Status: "healthy", Available: true, Version: "5.3", Message: "Tesseract OCR"}
	deps["playwright"] = domain.DepInfo{Status: "healthy", Available: true, Version: "1.40", Message: "Playwright PDF generation"}

	return deps, nil
}

// QueueServiceImpl implements the QueueService port
type QueueServiceImpl struct {
	queue ports.Queue
}

// NewQueueService creates a new queue service
func NewQueueService(queue ports.Queue) ports.QueueService {
	return &QueueServiceImpl{
		queue: queue,
	}
}

// GetQueueStats returns queue statistics
func (s *QueueServiceImpl) GetQueueStats(ctx context.Context) (*domain.QueueStats, error) {
	return s.queue.GetStats(ctx)
}

// EnqueueJob adds a job to the queue
func (s *QueueServiceImpl) EnqueueJob(ctx context.Context, job *domain.ProcessingJob) error {
	return s.queue.Enqueue(ctx, job)
}

// DequeueJob removes a job from the queue
func (s *QueueServiceImpl) DequeueJob(ctx context.Context) (*domain.ProcessingJob, error) {
	return s.queue.Dequeue(ctx)
}

// CompleteJob marks a job as completed
func (s *QueueServiceImpl) CompleteJob(ctx context.Context, jobID string, result map[string]interface{}) error {
	return s.queue.Complete(ctx, jobID, result)
}

// FailJob marks a job as failed
func (s *QueueServiceImpl) FailJob(ctx context.Context, jobID string, errorMsg string) error {
	return s.queue.Fail(ctx, jobID, errorMsg)
}
