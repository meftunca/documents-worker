package domain

import (
	"time"
)

// Document represents a document in the system
type Document struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        DocumentType           `json:"type"`
	Path        string                 `json:"path"`
	Size        int64                  `json:"size"`
	MimeType    string                 `json:"mime_type"`
	Status      DocumentStatus         `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
}

// DocumentType represents the type of document
type DocumentType string

const (
	DocumentTypePDF     DocumentType = "pdf"
	DocumentTypeImage   DocumentType = "image"
	DocumentTypeVideo   DocumentType = "video"
	DocumentTypeOffice  DocumentType = "office"
	DocumentTypeText    DocumentType = "text"
	DocumentTypeArchive DocumentType = "archive"
)

// DocumentStatus represents the processing status
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusCompleted  DocumentStatus = "completed"
	DocumentStatusFailed     DocumentStatus = "failed"
)

// ProcessingJob represents a document processing job
type ProcessingJob struct {
	ID          string                 `json:"id"`
	DocumentID  string                 `json:"document_id"`
	Type        ProcessingType         `json:"type"`
	Status      JobStatus              `json:"status"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// ProcessingType represents the type of processing
type ProcessingType string

const (
	ProcessingTypeOCR          ProcessingType = "ocr"
	ProcessingTypeImageConvert ProcessingType = "image_convert"
	ProcessingTypeVideoConvert ProcessingType = "video_convert"
	ProcessingTypePDFGenerate  ProcessingType = "pdf_generate"
	ProcessingTypeTextExtract  ProcessingType = "text_extract"
	ProcessingTypeThumbnail    ProcessingType = "thumbnail"
)

// JobStatus represents the job processing status
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusRetrying   JobStatus = "retrying"
)

// ProcessingRequest represents a request for document processing
type ProcessingRequest struct {
	DocumentID string                 `json:"document_id"`
	Type       ProcessingType         `json:"type"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Priority   int                    `json:"priority,omitempty"`
}

// ProcessingResult represents the result of document processing
type ProcessingResult struct {
	JobID       string                 `json:"job_id"`
	DocumentID  string                 `json:"document_id"`
	Type        ProcessingType         `json:"type"`
	Status      JobStatus              `json:"status"`
	OutputPath  string                 `json:"output_path,omitempty"`
	OutputSize  int64                  `json:"output_size,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	CompletedAt time.Time              `json:"completed_at"`
}

// HealthStatus represents system health status
type HealthStatus struct {
	Status       string                 `json:"status"`
	Version      string                 `json:"version"`
	Timestamp    time.Time              `json:"timestamp"`
	Services     map[string]ServiceInfo `json:"services"`
	Dependencies map[string]DepInfo     `json:"dependencies"`
}

// ServiceInfo represents information about a service
type ServiceInfo struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// DepInfo represents information about a dependency
type DepInfo struct {
	Status    string `json:"status"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Message   string `json:"message,omitempty"`
}

// QueueStats represents queue statistics
type QueueStats struct {
	PendingJobs    int64     `json:"pending_jobs"`
	ProcessingJobs int64     `json:"processing_jobs"`
	CompletedJobs  int64     `json:"completed_jobs"`
	FailedJobs     int64     `json:"failed_jobs"`
	TotalJobs      int64     `json:"total_jobs"`
	Timestamp      time.Time `json:"timestamp"`
}

// Error types
type DomainError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e DomainError) Error() string {
	return e.Message
}

// Common errors
var (
	ErrDocumentNotFound    = DomainError{Code: "DOCUMENT_NOT_FOUND", Message: "Document not found"}
	ErrJobNotFound         = DomainError{Code: "JOB_NOT_FOUND", Message: "Job not found"}
	ErrInvalidDocumentType = DomainError{Code: "INVALID_DOCUMENT_TYPE", Message: "Invalid document type"}
	ErrProcessingFailed    = DomainError{Code: "PROCESSING_FAILED", Message: "Document processing failed"}
	ErrUnsupportedFormat   = DomainError{Code: "UNSUPPORTED_FORMAT", Message: "Unsupported file format"}
)
