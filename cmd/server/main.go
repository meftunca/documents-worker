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
	"documents-worker/pkg/errors"
	"documents-worker/pkg/logger"
	"documents-worker/pkg/metrics"
	"documents-worker/pkg/validator"
	"documents-worker/queue"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize v2.0 structured logging
	loggerConfig := &logger.Config{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		Filename:   cfg.Logging.Filename,
		TimeFormat: cfg.Logging.TimeFormat,
	}

	if err := logger.Init(loggerConfig); err != nil {
		// Fallback to basic logging
		fmt.Printf("❌ Failed to initialize structured logger: %v, using default\n", err)
	}

	log := logger.Get()
	ctx := logger.WithCorrelationID(context.Background())

	log.FromContext(ctx).Info().Msg("🚀 Starting Documents Worker Server v2.0.0")
	log.FromContext(ctx).Info().
		Str("environment", cfg.Server.Environment).
		Str("port", cfg.Server.Port).
		Msg("📍 Configuration loaded")

	// Initialize v2.0 metrics
	if cfg.Metrics.Enabled {
		metrics.Init(cfg.Metrics.Namespace, cfg.Metrics.Subsystem)
		log.FromContext(ctx).Info().
			Str("port", cfg.Metrics.Port).
			Str("path", cfg.Metrics.Path).
			Msg("📊 Metrics initialized")
	}

	// Initialize v2.0 validation
	validatorConfig := &validator.Config{
		MaxFileSize:        cfg.Validation.MaxFileSize,
		MinFileSize:        cfg.Validation.MinFileSize,
		AllowedMimeTypes:   cfg.Validation.AllowedMimeTypes,
		AllowedExtensions:  cfg.Validation.AllowedExtensions,
		MaxConcurrentReqs:  cfg.Validation.MaxConcurrentReqs,
		MaxProcessingTime:  cfg.Validation.MaxProcessingTime,
		RequireContentType: cfg.Validation.RequireContentType,
		ScanForMalware:     cfg.Validation.ScanForMalware,
		MaxChunkSize:       cfg.Validation.MaxChunkSize,
		MinChunkSize:       cfg.Validation.MinChunkSize,
		MaxChunkOverlap:    cfg.Validation.MaxChunkOverlap,
	}
	validator.Init(validatorConfig)
	log.FromContext(ctx).Info().Msg("✅ Input validation initialized")

	// Initialize dependencies
	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
	if err != nil {
		log.FromContext(ctx).Fatal().Err(err).Msg("❌ Failed to initialize Redis queue")
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

	// Create Fiber app with v2.0 error handling
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Use v2.0 error handling
			if appErr, ok := err.(*errors.AppError); ok {
				return c.Status(appErr.HTTPStatus).JSON(errors.NewErrorResponse(appErr))
			}

			// Handle unknown errors
			internalErr := errors.NewInternalError(err.Error())
			return c.Status(internalErr.HTTPStatus).JSON(errors.NewErrorResponse(internalErr))
		},
		BodyLimit: int(cfg.Security.MaxRequestBodySize),
	})

	// v2.0 Middleware with structured logging and metrics
	app.Use(recover.New(recover.Config{
		EnableStackTrace: !cfg.IsProduction(),
	}))

	// Custom logging middleware
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		reqCtx := logger.WithRequestID(c.Context(), requestID)

		err := c.Next()

		duration := time.Since(start)

		// Log request with context
		log.LogRequest(reqCtx, c.Method(), c.Path(), c.Get("User-Agent"), c.IP(), duration)

		// Record metrics
		if cfg.Metrics.Enabled {
			statusCode := fmt.Sprintf("%d", c.Response().StatusCode())
			metrics.Get().RecordHTTPRequest(c.Method(), c.Path(), statusCode, duration, int64(len(c.Response().Body())))
		}

		return err
	})

	// v2.0 Rate limiting
	if cfg.Security.RateLimitEnabled {
		app.Use(limiter.New(limiter.Config{
			Max:        cfg.Security.RateLimitPerMinute,
			Expiration: 1 * time.Minute,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return errors.NewRateLimitError("Rate limit exceeded")
			},
		}))
	}

	// v2.0 CORS with configuration
	if cfg.Security.CorsEnabled {
		app.Use(cors.New(cors.Config{
			AllowOrigins: func() string {
				if len(cfg.Security.CorsAllowedOrigins) > 0 {
					return cfg.Security.CorsAllowedOrigins[0]
				}
				return "*"
			}(),
			AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
			AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Request-ID",
		}))
	}

	// Setup routes
	httpHandler.SetupRoutes(app)

	// v2.0 Health check endpoints
	if cfg.Health.Enabled {
		healthChecker := health.NewHealthChecker(cfg, redisQueue)

		app.Get(cfg.Health.Path, func(c *fiber.Ctx) error {
			status := healthChecker.GetHealthStatus()
			httpStatus := fiber.StatusOK
			if status.Status != "healthy" {
				httpStatus = fiber.StatusServiceUnavailable
			}
			return c.Status(httpStatus).JSON(status)
		})

		app.Get(cfg.Health.ReadinessPath, func(c *fiber.Ctx) error {
			// Readiness check - can the app handle traffic?
			return c.JSON(fiber.Map{"status": "ready"})
		})

		app.Get(cfg.Health.LivenessPath, func(c *fiber.Ctx) error {
			// Liveness check - is the app running?
			return c.JSON(fiber.Map{"status": "alive"})
		})
	}

	// v2.0 Metrics endpoint
	if cfg.Metrics.Enabled {
		// Start metrics server
		go func() {
			metricsApp := fiber.New()
			metricsApp.Get(cfg.Metrics.Path, adaptor.HTTPHandler(promhttp.Handler()))

			log.FromContext(ctx).Info().
				Str("port", cfg.Metrics.Port).
				Msg("📊 Metrics server starting")

			if err := metricsApp.Listen(":" + cfg.Metrics.Port); err != nil {
				log.FromContext(ctx).Error().Err(err).Msg("❌ Failed to start metrics server")
			}
		}()
	}

	// Start main server
	go func() {
		log.FromContext(ctx).Info().
			Str("port", cfg.Server.Port).
			Msg("🌐 HTTP Server starting")

		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.FromContext(ctx).Fatal().Err(err).Msg("❌ Failed to start HTTP server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.FromContext(ctx).Info().Msg("🛑 Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.FromContext(ctx).Error().Err(err).Msg("❌ Server shutdown error")
	}

	log.FromContext(ctx).Info().Msg("✅ Server stopped")
}
