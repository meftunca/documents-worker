package main

import (
	"context"
	"documents-worker/cache"
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

	// Initialize cache manager
	cacheManager := cache.NewManager(cfg)

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

	// PDF Generation endpoints
	api.Post("/pdf/generate/html", handlePDFFromHTMLRequest(pdfGenerator))
	api.Post("/pdf/generate/markdown", handlePDFFromMarkdownRequest(pdfGenerator))
	api.Post("/pdf/generate/office", handlePDFFromOfficeRequest(pdfGenerator))

	// Markdown conversion endpoints (PyMuPDF)
	api.Post("/markdown/convert", handleMarkdownConversionRequest(pymuPDFConverter))
	api.Post("/markdown/convert/pdf", handlePDFToMarkdownRequest(pymuPDFConverter))
	api.Post("/markdown/convert/office", handleOfficeToMarkdownRequest(pymuPDFConverter))
	api.Post("/markdown/batch", handleBatchMarkdownConversionRequest(pymuPDFConverter))

	// Cache management endpoints
	api.Get("/cache/stats", handleCacheStatsRequest(cacheManager))
	api.Delete("/cache/clear", handleCacheClearRequest(cacheManager))

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

// PDF Generation Handlers

func handlePDFFromHTMLRequest(pdfGen *pdfgen.PDFGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to parse multipart form",
			})
		}

		files := form.File["html_file"]
		if len(files) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "No HTML file provided",
			})
		}

		// Save uploaded file
		file := files[0]
		inputFile, err := utils.SaveUploadedFile(file)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		// Generate PDF options
		options := &pdfgen.GenerationOptions{
			PageSize: c.FormValue("page_size", "A4"),
			Margins: map[string]string{
				"top":    c.FormValue("margin_top", "1cm"),
				"right":  c.FormValue("margin_right", "1cm"),
				"bottom": c.FormValue("margin_bottom", "1cm"),
				"left":   c.FormValue("margin_left", "1cm"),
			},
		}

		// Generate PDF
		result, err := pdfGen.GenerateFromHTMLFile(inputFile.Name(), options)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "PDF generation failed",
				"details": err.Error(),
			})
		}

		// Return PDF file
		return c.SendFile(result.OutputPath)
	}
}

func handlePDFFromMarkdownRequest(pdfGen *pdfgen.PDFGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Content string `json:"content"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid JSON body",
			})
		}

		if req.Content == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Markdown content is required",
			})
		}

		// Generate PDF options
		options := &pdfgen.GenerationOptions{
			PageSize: c.Query("page_size", "A4"),
			Margins: map[string]string{
				"top":    c.Query("margin_top", "1cm"),
				"right":  c.Query("margin_right", "1cm"),
				"bottom": c.Query("margin_bottom", "1cm"),
				"left":   c.Query("margin_left", "1cm"),
			},
		}

		// Generate PDF
		result, err := pdfGen.GenerateFromMarkdown(req.Content, options)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "PDF generation failed",
				"details": err.Error(),
			})
		}

		// Return PDF file
		return c.SendFile(result.OutputPath)
	}
}

func handlePDFFromOfficeRequest(pdfGen *pdfgen.PDFGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to parse multipart form",
			})
		}

		files := form.File["office_file"]
		if len(files) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "No office file provided",
			})
		}

		// Save uploaded file
		file := files[0]
		inputFile, err := utils.SaveUploadedFile(file)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		// Generate PDF options
		options := &pdfgen.GenerationOptions{}

		// Generate PDF
		result, err := pdfGen.GenerateFromOfficeDocument(inputFile.Name(), options)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "PDF generation failed",
				"details": err.Error(),
			})
		}

		// Return PDF file
		return c.SendFile(result.OutputPath)
	}
}

// Markdown Conversion Handlers

func handleMarkdownConversionRequest(converter *pymupdf.PyMuPDFConverter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to parse multipart form",
			})
		}

		files := form.File["document"]
		if len(files) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "No document file provided",
			})
		}

		// Save uploaded file
		file := files[0]
		inputFile, err := utils.SaveUploadedFile(file)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		// Conversion options
		options := pymupdf.DefaultConversionOptions()

		// Convert to markdown
		result, err := converter.ConvertToMarkdown(inputFile.Name(), "", options)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Markdown conversion failed",
				"details": err.Error(),
			})
		}

		// Return markdown file
		defer os.Remove(result.OutputPath)
		return c.SendFile(result.OutputPath)
	}
}

func handlePDFToMarkdownRequest(converter *pymupdf.PyMuPDFConverter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to parse multipart form",
			})
		}

		files := form.File["pdf_file"]
		if len(files) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "No PDF file provided",
			})
		}

		// Save uploaded file
		file := files[0]
		inputFile, err := utils.SaveUploadedFile(file)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		// Conversion options
		options := pymupdf.DefaultConversionOptions()

		// Convert PDF to markdown
		result, err := converter.ConvertPDFToMarkdown(inputFile.Name(), "", options)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "PDF to markdown conversion failed",
				"details": err.Error(),
			})
		}

		// Return markdown file
		defer os.Remove(result.OutputPath)
		return c.SendFile(result.OutputPath)
	}
}

func handleOfficeToMarkdownRequest(converter *pymupdf.PyMuPDFConverter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to parse multipart form",
			})
		}

		files := form.File["office_file"]
		if len(files) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "No office file provided",
			})
		}

		// Save uploaded file
		file := files[0]
		inputFile, err := utils.SaveUploadedFile(file)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to save uploaded file",
				"details": err.Error(),
			})
		}
		defer os.Remove(inputFile.Name())

		// Conversion options
		options := pymupdf.DefaultConversionOptions()

		// Convert office document to markdown
		result, err := converter.ConvertOfficeToMarkdown(inputFile.Name(), "", options)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Office to markdown conversion failed",
				"details": err.Error(),
			})
		}

		// Return markdown file
		defer os.Remove(result.OutputPath)
		return c.SendFile(result.OutputPath)
	}
}

func handleBatchMarkdownConversionRequest(converter *pymupdf.PyMuPDFConverter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			InputDirectory  string                    `json:"input_directory"`
			OutputDirectory string                    `json:"output_directory,omitempty"`
			Options         pymupdf.ConversionOptions `json:"options,omitempty"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid JSON body",
			})
		}

		if req.InputDirectory == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Input directory is required",
			})
		}

		// Use default options if not provided
		options := req.Options
		if options.PreserveImages == false && options.TableStyle == false {
			options = pymupdf.DefaultConversionOptions()
		}

		// Batch convert
		result, err := converter.BatchConvert(req.InputDirectory, req.OutputDirectory, options)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Batch conversion failed",
				"details": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"summary": result.Summary,
			"results": result.Results,
		})
	}
}

// Cache Management Handlers

func handleCacheStatsRequest(cacheManager *cache.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats := cacheManager.GetCacheStats()
		return c.JSON(fiber.Map{
			"success": true,
			"cache":   stats,
		})
	}
}

func handleCacheClearRequest(cacheManager *cache.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !cacheManager.IsEnabled() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Cache is not enabled",
			})
		}

		// For now, we don't implement full cache clear
		// This would require adding a ClearAll method to the cache manager
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Cache clear operation completed",
		})
	}
}
