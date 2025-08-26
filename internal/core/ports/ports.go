package ports

import (
	"context"
	"documents-worker/internal/core/domain"
	"io"
)

// Primary Ports (inbound)

// DocumentService defines the core document processing operations
type DocumentService interface {
	// Document management
	ProcessDocument(ctx context.Context, req *domain.ProcessingRequest) (*domain.ProcessingResult, error)
	GetDocument(ctx context.Context, id string) (*domain.Document, error)
	GetJob(ctx context.Context, jobID string) (*domain.ProcessingJob, error)
	GetJobsByDocument(ctx context.Context, documentID string) ([]*domain.ProcessingJob, error)

	// Processing operations
	ConvertImage(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error)
	ConvertVideo(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error)
	GeneratePDF(ctx context.Context, input io.Reader, params map[string]interface{}) (io.Reader, error)
	ExtractText(ctx context.Context, input io.Reader, docType domain.DocumentType) (string, error)
	PerformOCR(ctx context.Context, input io.Reader, language string) (string, error)
	GenerateThumbnail(ctx context.Context, input io.Reader, params map[string]interface{}) (io.Reader, error)
}

// HealthService defines health checking operations
type HealthService interface {
	GetHealthStatus(ctx context.Context) (*domain.HealthStatus, error)
	CheckDependencies(ctx context.Context) (map[string]domain.DepInfo, error)
}

// QueueService defines queue management operations
type QueueService interface {
	GetQueueStats(ctx context.Context) (*domain.QueueStats, error)
	EnqueueJob(ctx context.Context, job *domain.ProcessingJob) error
	DequeueJob(ctx context.Context) (*domain.ProcessingJob, error)
	CompleteJob(ctx context.Context, jobID string, result map[string]interface{}) error
	FailJob(ctx context.Context, jobID string, errorMsg string) error
}

// Secondary Ports (outbound)

// DocumentRepository defines document persistence operations
type DocumentRepository interface {
	Save(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, id string) (*domain.Document, error)
	Update(ctx context.Context, doc *domain.Document) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*domain.Document, error)
}

// JobRepository defines job persistence operations
type JobRepository interface {
	Save(ctx context.Context, job *domain.ProcessingJob) error
	GetByID(ctx context.Context, id string) (*domain.ProcessingJob, error)
	GetByDocumentID(ctx context.Context, documentID string) ([]*domain.ProcessingJob, error)
	Update(ctx context.Context, job *domain.ProcessingJob) error
	Delete(ctx context.Context, id string) error
	ListPending(ctx context.Context, limit int) ([]*domain.ProcessingJob, error)
}

// FileStorage defines file storage operations
type FileStorage interface {
	Store(ctx context.Context, path string, data io.Reader) error
	Retrieve(ctx context.Context, path string) (io.Reader, error)
	Delete(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	GetMetadata(ctx context.Context, path string) (map[string]interface{}, error)
}

// Queue defines queue operations
type Queue interface {
	Enqueue(ctx context.Context, job *domain.ProcessingJob) error
	Dequeue(ctx context.Context) (*domain.ProcessingJob, error)
	Complete(ctx context.Context, jobID string, result map[string]interface{}) error
	Fail(ctx context.Context, jobID string, errorMsg string) error
	GetStats(ctx context.Context) (*domain.QueueStats, error)
	Close() error
}

// Cache defines caching operations
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl int64) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Close() error
}

// Document Processors

// ImageProcessor defines image processing operations
type ImageProcessor interface {
	Convert(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error)
	Resize(ctx context.Context, input io.Reader, width, height int, params map[string]interface{}) (io.Reader, error)
	GenerateThumbnail(ctx context.Context, input io.Reader, size int) (io.Reader, error)
}

// VideoProcessor defines video processing operations
type VideoProcessor interface {
	Convert(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error)
	GenerateThumbnail(ctx context.Context, input io.Reader, timeOffset int) (io.Reader, error)
	Compress(ctx context.Context, input io.Reader, quality int) (io.Reader, error)
}

// PDFProcessor defines PDF processing operations
type PDFProcessor interface {
	GenerateFromHTML(ctx context.Context, html io.Reader, params map[string]interface{}) (io.Reader, error)
	GenerateFromURL(ctx context.Context, url string, params map[string]interface{}) (io.Reader, error)
	ExtractText(ctx context.Context, input io.Reader) (string, error)
	GetPageCount(ctx context.Context, input io.Reader) (int, error)
}

// OCRProcessor defines OCR processing operations
type OCRProcessor interface {
	ProcessImage(ctx context.Context, input io.Reader, language string) (string, error)
	ProcessPDF(ctx context.Context, input io.Reader, language string) (string, error)
	GetSupportedLanguages() []string
}

// TextExtractor defines text extraction operations
type TextExtractor interface {
	ExtractFromOffice(ctx context.Context, input io.Reader, docType string) (string, error)
	ExtractFromPDF(ctx context.Context, input io.Reader) (string, error)
	ExtractFromText(ctx context.Context, input io.Reader) (string, error)
}

// EventPublisher defines event publishing operations
type EventPublisher interface {
	PublishDocumentProcessed(ctx context.Context, event *DocumentProcessedEvent) error
	PublishJobCompleted(ctx context.Context, event *JobCompletedEvent) error
	PublishJobFailed(ctx context.Context, event *JobFailedEvent) error
}

// Event types
type DocumentProcessedEvent struct {
	DocumentID  string                 `json:"document_id"`
	Type        domain.ProcessingType  `json:"type"`
	Status      domain.JobStatus       `json:"status"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ProcessedAt string                 `json:"processed_at"`
}

type JobCompletedEvent struct {
	JobID       string                 `json:"job_id"`
	DocumentID  string                 `json:"document_id"`
	Type        domain.ProcessingType  `json:"type"`
	Result      map[string]interface{} `json:"result"`
	Duration    string                 `json:"duration"`
	CompletedAt string                 `json:"completed_at"`
}

type JobFailedEvent struct {
	JobID      string                `json:"job_id"`
	DocumentID string                `json:"document_id"`
	Type       domain.ProcessingType `json:"type"`
	Error      string                `json:"error"`
	RetryCount int                   `json:"retry_count"`
	FailedAt   string                `json:"failed_at"`
}
