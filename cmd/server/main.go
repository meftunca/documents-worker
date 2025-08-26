package main

import (
	"context"
	"documents-worker/cache"
	"documents-worker/config"
	"documents-worker/health"
	"documents-worker/internal/adapters/primary/http"
	adapters "documents-worker/internal/adapters/secondary"
	"documents-worker/internal/adapters/secondary/processors"
	"documents-worker/internal/core/services"
	"documents-worker/queue"
	"log"
	"os"
	"os/signal"
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

	log.Printf("🚀 Starting Documents Worker Server v1.0.0")
	log.Printf("📍 Environment: %s", cfg.Server.Environment)
	log.Printf("🌐 Port: %s", cfg.Server.Port)

	// Initialize dependencies
	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Redis queue: %v", err)
	}
	defer redisQueue.Close()

	cacheManager := cache.NewCacheManager(cfg.Cache.Directory, cfg.Cache.TTL, cfg.Cache.Enabled)

	// Create adapters for legacy components
	queueAdapter := adapters.NewQueueAdapter(redisQueue)
	cacheAdapter := adapters.NewCacheAdapter(cacheManager)

	// Initialize processors (secondary adapters)
	imageProcessor := processors.NewVipsImageProcessor()
	videoProcessor := processors.NewFFmpegVideoProcessor()
	pdfProcessor := processors.NewPlaywrightPDFProcessor(&cfg.External)
	ocrProcessor := processors.NewTesseractOCRProcessor(&cfg.OCR, &cfg.External)
	textExtractor := processors.NewMultiTextExtractor(&cfg.External)

	// Initialize core services
	documentService := services.NewDocumentService(
		nil, // documentRepo - would be implemented for persistence
		nil, // jobRepo - would be implemented for persistence
		nil, // fileStorage - would be implemented for file storage
		queueAdapter,
		imageProcessor,
		videoProcessor,
		pdfProcessor,
		ocrProcessor,
		textExtractor,
		nil, // eventPublisher - would be implemented for events
	)

	healthService := services.NewHealthService(
		queueAdapter,
		cacheAdapter,
		nil, // fileStorage
		imageProcessor,
		videoProcessor,
		pdfProcessor,
		ocrProcessor,
	)

	queueService := services.NewQueueService(queueAdapter)

	// Initialize HTTP adapter (primary adapter)
	httpHandler := http.NewDocumentHandler(documentService, healthService, queueService)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error":   err.Error(),
				"code":    code,
				"success": false,
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${method} ${path} ${status} ${latency}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Setup routes
	httpHandler.SetupRoutes(app)

	// Health check endpoint
	healthChecker := health.NewHealthChecker(cfg, redisQueue)
	app.Get("/health", func(c *fiber.Ctx) error {
		status := healthChecker.GetHealthStatus()
		httpStatus := fiber.StatusOK
		if status.Status != "healthy" {
			httpStatus = fiber.StatusServiceUnavailable
		}
		return c.Status(httpStatus).JSON(status)
	})

	// Start server in goroutine
	go func() {
		log.Printf("🌐 HTTP Server starting on port %s", cfg.Server.Port)
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Fatalf("❌ Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("❌ Server shutdown error: %v", err)
	}

	log.Println("✅ Server stopped")
}
