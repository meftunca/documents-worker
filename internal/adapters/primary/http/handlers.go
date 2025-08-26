package http

import (
	"documents-worker/internal/core/domain"
	"documents-worker/internal/core/ports"

	"github.com/gofiber/fiber/v2"
)

// DocumentHandler handles HTTP requests for document operations
type DocumentHandler struct {
	documentService ports.DocumentService
	healthService   ports.HealthService
	queueService    ports.QueueService
}

// NewDocumentHandler creates a new document handler
func NewDocumentHandler(
	documentService ports.DocumentService,
	healthService ports.HealthService,
	queueService ports.QueueService,
) *DocumentHandler {
	return &DocumentHandler{
		documentService: documentService,
		healthService:   healthService,
		queueService:    queueService,
	}
}

// ProcessDocumentRequest represents a document processing request
type ProcessDocumentRequest struct {
	DocumentID string                 `json:"document_id" validate:"required"`
	Type       domain.ProcessingType  `json:"type" validate:"required"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Priority   int                    `json:"priority,omitempty"`
}

// ProcessDocument handles document processing requests
func (h *DocumentHandler) ProcessDocument(c *fiber.Ctx) error {
	var req ProcessDocumentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
	}

	processingReq := &domain.ProcessingRequest{
		DocumentID: req.DocumentID,
		Type:       req.Type,
		Parameters: req.Parameters,
		Priority:   req.Priority,
	}

	result, err := h.documentService.ProcessDocument(c.Context(), processingReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to process document",
			"details": err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(result)
}

// GetDocument retrieves a document by ID
func (h *DocumentHandler) GetDocument(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Document ID is required",
		})
	}

	doc, err := h.documentService.GetDocument(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Document not found",
			"details": err.Error(),
		})
	}

	return c.JSON(doc)
}

// GetJob retrieves a job by ID
func (h *DocumentHandler) GetJob(c *fiber.Ctx) error {
	jobID := c.Params("jobId")
	if jobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Job ID is required",
		})
	}

	job, err := h.documentService.GetJob(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Job not found",
			"details": err.Error(),
		})
	}

	return c.JSON(job)
}

// GetJobsByDocument retrieves all jobs for a document
func (h *DocumentHandler) GetJobsByDocument(c *fiber.Ctx) error {
	documentID := c.Params("id")
	if documentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Document ID is required",
		})
	}

	jobs, err := h.documentService.GetJobsByDocument(c.Context(), documentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get jobs",
			"details": err.Error(),
		})
	}

	return c.JSON(jobs)
}

// ConvertImageRequest represents an image conversion request
type ConvertImageRequest struct {
	OutputFormat string                 `json:"output_format" validate:"required"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

// ConvertImage handles image conversion requests
func (h *DocumentHandler) ConvertImage(c *fiber.Ctx) error {
	var req ConvertImageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
	}

	// Get file from multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "No file provided",
			"details": err.Error(),
		})
	}

	// Open file
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to open file",
			"details": err.Error(),
		})
	}
	defer src.Close()

	// Convert image
	result, err := h.documentService.ConvertImage(c.Context(), src, req.OutputFormat, req.Parameters)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to convert image",
			"details": err.Error(),
		})
	}

	// Set appropriate content type
	contentType := "application/octet-stream"
	switch req.OutputFormat {
	case "jpg", "jpeg":
		contentType = "image/jpeg"
	case "png":
		contentType = "image/png"
	case "webp":
		contentType = "image/webp"
	case "avif":
		contentType = "image/avif"
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "attachment; filename=\"converted."+req.OutputFormat+"\"")

	return c.SendStream(result)
}

// HealthCheck handles health check requests
func (h *DocumentHandler) HealthCheck(c *fiber.Ctx) error {
	health, err := h.healthService.GetHealthStatus(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get health status",
			"details": err.Error(),
		})
	}

	status := fiber.StatusOK
	if health.Status != "healthy" {
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(health)
}

// GetQueueStats handles queue statistics requests
func (h *DocumentHandler) GetQueueStats(c *fiber.Ctx) error {
	stats, err := h.queueService.GetQueueStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get queue stats",
			"details": err.Error(),
		})
	}

	return c.JSON(stats)
}

// SetupRoutes configures the HTTP routes
func (h *DocumentHandler) SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	// Health endpoints
	api.Get("/health", h.HealthCheck)
	api.Get("/stats/queue", h.GetQueueStats)

	// Document endpoints
	documents := api.Group("/documents")
	documents.Post("/process", h.ProcessDocument)
	documents.Get("/:id", h.GetDocument)
	documents.Get("/:id/jobs", h.GetJobsByDocument)

	// Job endpoints
	jobs := api.Group("/jobs")
	jobs.Get("/:jobId", h.GetJob)

	// Processing endpoints
	processing := api.Group("/process")
	processing.Post("/image/convert", h.ConvertImage)
	// Add more processing endpoints here
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	Code    string `json:"code,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}
