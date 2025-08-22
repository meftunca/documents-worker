package main

import (
	"context"
	"documents-worker/config"
	"documents-worker/health"
	"documents-worker/media"
	"documents-worker/ocr"
	"documents-worker/pdfgen"
	"documents-worker/pymupdf"
	"documents-worker/queue"
	"documents-worker/textextractor"
	"documents-worker/types"
	"documents-worker/utils"
	"documents-worker/worker"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Load configuration
	cfg := config.Load()

	log.Printf("Starting Documents Worker v1.0.0")
	log.Printf("Environment: %s", cfg.Server.Environment)
	log.Printf("Port: %s", cfg.Server.Port)

	// Initialize Redis queue
	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	if err != nil {
		log.Fatalf("Failed to initialize Redis queue: %v", err)
	}
	defer redisQueue.Close()

	// Initialize health checker
	healthChecker := health.NewHealthChecker(cfg, redisQueue)

	// Initialize OCR processor
	ocrProcessor := ocr.NewOCRProcessor(&cfg.OCR, &cfg.External)

	// Initialize text extractor
	textExtractor := textextractor.NewTextExtractor(&cfg.External)

	// Initialize PDF generator
	pdfGenerator := pdfgen.NewPDFGenerator(&cfg.External)

	// Initialize PyMuPDF converter
	pymuPDFConverter := pymupdf.NewPyMuPDFConverter(cfg.External.PyMuPDFScript)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
		ErrorHandler: errorHandler,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} - ${latency}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Health check endpoints (for Kubernetes)
	app.Get("/health", healthChecker.HealthHandler)
	app.Get("/health/readiness", healthChecker.ReadinessHandler)
	app.Get("/health/liveness", healthChecker.LivenessHandler)
	app.Get("/metrics", healthChecker.MetricsHandler)

	// API routes
	api := app.Group("/api/v1")

	// Async processing endpoints
	api.Post("/process/document", handleAsyncRequest(redisQueue, types.DocKind))
	api.Post("/process/image", handleAsyncRequest(redisQueue, types.ImageKind))
	api.Post("/process/video", handleAsyncRequest(redisQueue, types.VideoKind))

	// OCR endpoints
	api.Post("/ocr/image", handleOCRImageRequest(ocrProcessor))
	api.Post("/ocr/document", handleOCRDocumentRequest(ocrProcessor))

	// Text extraction endpoints (synchronous)
	api.Post("/extract/text", handleTextExtractionRequest(textExtractor))
	api.Post("/extract/pdf-pages", handlePDFPagesExtractionRequest(textExtractor))
	api.Post("/extract/pdf-range", handlePDFRangeExtractionRequest(textExtractor))

	// Asynchronous text extraction endpoints
	api.Post("/extract/async/text", handleAsyncTextExtractionRequest(redisQueue, "full"))
	api.Post("/extract/async/pdf-pages", handleAsyncTextExtractionRequest(redisQueue, "pages"))
	api.Post("/extract/async/pdf-range", handleAsyncTextExtractionRangeRequest(redisQueue))

	// Job status endpoints
	api.Get("/job/:id", handleJobStatus(redisQueue))
	api.Get("/queue/stats", handleQueueStats(redisQueue))

	// Synchronous endpoints (for backward compatibility)
	sync := api.Group("/sync")
	sync.Post("/convert/document", handleSyncRequest(types.DocKind, cfg))
	sync.Post("/convert/image", handleSyncRequest(types.ImageKind, cfg))
	sync.Post("/convert/video", handleSyncRequest(types.VideoKind, cfg))

	// Start worker
	workerInstance := worker.NewWorker(redisQueue, cfg)
	workerInstance.Start()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Gracefully shutting down...")
		workerInstance.Stop()
		app.Shutdown()
	}()

	// Start server
	log.Printf("Server starting on port %s", cfg.Server.Port)
	if err := app.Listen(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleAsyncRequest(redisQueue *queue.RedisQueue, kind types.MediaKind) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get uploaded file
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		// Save uploaded file
		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}

		// Parse query parameters
		searchParams := parseSearchParams(c)
		format := parseFormat(c)
		vipsEnabled := c.Query("vipsEnable", "true") != "false"

		// Create metadata
		metadata := map[string]interface{}{
			"original_filename": fileHeader.Filename,
			"file_size":         fileHeader.Size,
			"upload_time":       time.Now(),
			"client_ip":         c.IP(),
		}

		// Submit job to queue
		job, err := worker.SubmitMediaJob(
			redisQueue,
			inputFile.Name(),
			kind,
			searchParams,
			format,
			vipsEnabled,
			metadata,
		)
		if err != nil {
			os.Remove(inputFile.Name())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to submit job",
				"details": err.Error(),
			})
		}

		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"job_id":           job.ID,
			"status":           "accepted",
			"message":          "Job submitted for processing",
			"check_status_url": "/api/v1/job/" + job.ID,
		})
	}
}

func handleSyncRequest(kind types.MediaKind, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// This is the original synchronous processing for backward compatibility
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File upload error"})
		}

		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save file"})
		}
		defer os.Remove(inputFile.Name())

		// Create media converter from request
		mediaConverter, err := createMediaConverterFromFiber(c, kind)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		// Process synchronously
		processor, err := media.NewProcessor(mediaConverter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create processor"})
		}

		outputFile, err := processor.Process(inputFile.Name())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Processing failed"})
		}
		defer os.Remove(outputFile.Name())

		return c.Download(outputFile.Name())
	}
}

func handleOCRImageRequest(ocrProcessor *ocr.OCRProcessor) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		result, err := ocrProcessor.ProcessImage(inputFile.Name())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "OCR processing failed",
				"details": err.Error(),
			})
		}

		return c.JSON(result)
	}
}

func handleOCRDocumentRequest(ocrProcessor *ocr.OCRProcessor) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		result, err := ocrProcessor.ProcessDocument(inputFile.Name())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "OCR processing failed",
				"details": err.Error(),
			})
		}

		return c.JSON(result)
	}
}

func handleJobStatus(redisQueue *queue.RedisQueue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		jobID := c.Params("id")
		if jobID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Job ID is required",
			})
		}

		job, err := redisQueue.GetJob(context.Background(), jobID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "Job not found",
				"details": err.Error(),
			})
		}

		return c.JSON(job)
	}
}

func handleQueueStats(redisQueue *queue.RedisQueue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := redisQueue.GetQueueStats(context.Background())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to get queue stats",
				"details": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"queue_stats": stats,
			"timestamp":   time.Now(),
		})
	}
}

func parseSearchParams(c *fiber.Ctx) types.MediaSearch {
	var search types.MediaSearch

	if width := c.Query("width"); width != "" {
		if w, err := parseIntParam(width); err == nil {
			search.Width = &w
		}
	}

	if height := c.Query("height"); height != "" {
		if h, err := parseIntParam(height); err == nil {
			search.Height = &h
		}
	}

	if crop := c.Query("crop"); crop != "" {
		search.Crop = &crop
	}

	if quality := c.Query("quality"); quality != "" {
		if q, err := parseIntParam(quality); err == nil {
			search.Quality = &q
		}
	}

	if resizeScale := c.Query("resize"); resizeScale != "" {
		if r, err := parseIntParam(resizeScale); err == nil {
			search.ResizeScale = &r
		}
	}

	if cutVideo := c.Query("clip"); cutVideo != "" {
		search.CutVideo = &cutVideo
	}

	if page := c.Query("page"); page != "" {
		if p, err := parseIntParam(page); err == nil && p > 0 {
			search.Page = &p
		}
	}

	return search
}

func parseFormat(c *fiber.Ctx) *string {
	if format := c.Query("format"); format != "" {
		return &format
	}
	return nil
}

func createMediaConverterFromFiber(c *fiber.Ctx, kind types.MediaKind) (*types.MediaConverter, error) {
	searchParams := parseSearchParams(c)
	format := parseFormat(c)
	vipsEnabled := c.Query("vipsEnable", "true") != "false"

	return &types.MediaConverter{
		Kind:        kind,
		Search:      searchParams,
		Format:      format,
		VipsEnabled: vipsEnabled,
	}, nil
}

func parseIntParam(s string) (int, error) {
	return strconv.Atoi(s)
}

// Text extraction handlers
func handleTextExtractionRequest(textExtractor *textextractor.TextExtractor) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		result, err := textExtractor.ExtractFromFile(inputFile.Name())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Text extraction failed",
				"details": err.Error(),
			})
		}

		return c.JSON(result)
	}
}

func handlePDFPagesExtractionRequest(textExtractor *textextractor.TextExtractor) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		// Verify it's a PDF
		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		// Check if it's a PDF
		mimeType, err := utils.DetectMimeTypeFromFile(inputFile.Name())
		if err != nil || !utils.IsPdfDocument(mimeType) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File must be a PDF document",
				"details": "Detected MIME type: " + mimeType,
			})
		}

		results, err := textExtractor.BatchExtractPDFPages(inputFile.Name())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "PDF batch extraction failed",
				"details": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"pages":        results,
			"total_pages":  len(results),
			"extracted_at": time.Now(),
		})
	}
}

func handlePDFRangeExtractionRequest(textExtractor *textextractor.TextExtractor) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		// Get page range parameters
		startPageStr := c.FormValue("start_page")
		endPageStr := c.FormValue("end_page")

		if startPageStr == "" || endPageStr == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "start_page and end_page parameters are required",
				"example": "start_page=1&end_page=3",
			})
		}

		startPage, err := strconv.Atoi(startPageStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid start_page parameter",
				"details": err.Error(),
			})
		}

		endPage, err := strconv.Atoi(endPageStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid end_page parameter",
				"details": err.Error(),
			})
		}

		if startPage < 1 || endPage < 1 || startPage > endPage {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid page range",
				"details": "start_page and end_page must be positive and start_page <= end_page",
			})
		}

		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		// Check if it's a PDF
		mimeType, err := utils.DetectMimeTypeFromFile(inputFile.Name())
		if err != nil || !utils.IsPdfDocument(mimeType) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File must be a PDF document",
				"details": "Detected MIME type: " + mimeType,
			})
		}

		result, err := textExtractor.ExtractByPages(inputFile.Name(), startPage, endPage)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "PDF range extraction failed",
				"details": err.Error(),
			})
		}

		return c.JSON(result)
	}
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error":     message,
		"timestamp": time.Now(),
		"path":      c.Path(),
	})
}

// Async text extraction handlers
func handleAsyncTextExtractionRequest(redisQueue *queue.RedisQueue, jobType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}

		// Create metadata
		metadata := map[string]interface{}{
			"original_filename": fileHeader.Filename,
			"file_size":         fileHeader.Size,
			"upload_time":       time.Now(),
			"client_ip":         c.IP(),
		}

		// Submit job to queue
		job, err := worker.SubmitTextExtractionJob(
			redisQueue,
			inputFile.Name(),
			jobType,
			nil, // no page range for full/pages extraction
			nil,
			metadata,
		)
		if err != nil {
			os.Remove(inputFile.Name())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to submit text extraction job",
				"details": err.Error(),
			})
		}

		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"job_id":           job.ID,
			"status":           "accepted",
			"job_type":         jobType,
			"message":          "Text extraction job submitted for processing",
			"check_status_url": "/api/v1/job/" + job.ID,
		})
	}
}

func handleAsyncTextExtractionRangeRequest(redisQueue *queue.RedisQueue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "File upload error",
				"details": err.Error(),
			})
		}

		// Get page range parameters
		startPageStr := c.FormValue("start_page")
		endPageStr := c.FormValue("end_page")

		if startPageStr == "" || endPageStr == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "start_page and end_page parameters are required",
				"example": "start_page=1&end_page=3",
			})
		}

		startPage, err := strconv.Atoi(startPageStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid start_page parameter",
				"details": err.Error(),
			})
		}

		endPage, err := strconv.Atoi(endPageStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid end_page parameter",
				"details": err.Error(),
			})
		}

		if startPage < 1 || endPage < 1 || startPage > endPage {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid page range",
				"details": "start_page and end_page must be positive and start_page <= end_page",
			})
		}

		inputFile, err := utils.SaveUploadedFile(fileHeader)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}

		// Create metadata
		metadata := map[string]interface{}{
			"original_filename": fileHeader.Filename,
			"file_size":         fileHeader.Size,
			"upload_time":       time.Now(),
			"client_ip":         c.IP(),
		}

		// Submit job to queue
		job, err := worker.SubmitTextExtractionJob(
			redisQueue,
			inputFile.Name(),
			"range",
			&startPage,
			&endPage,
			metadata,
		)
		if err != nil {
			os.Remove(inputFile.Name())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to submit text extraction job",
				"details": err.Error(),
			})
		}

		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"job_id":           job.ID,
			"status":           "accepted",
			"job_type":         "range",
			"start_page":       startPage,
			"end_page":         endPage,
			"message":          "Text extraction job submitted for processing",
			"check_status_url": "/api/v1/job/" + job.ID,
		})
	}
}
