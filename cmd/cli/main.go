package main

import (
	"documents-worker/config"
	"documents-worker/internal/adapters/primary/cli"
	adapters "documents-worker/internal/adapters/secondary"
	"documents-worker/internal/adapters/secondary/processors"
	"documents-worker/internal/core/ports"
	"documents-worker/internal/core/services"
	"documents-worker/queue"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	rootCmd = &cobra.Command{
		Use:   "documents-worker",
		Short: "A CLI tool for document processing",
		Long:  `Documents Worker CLI - Convert, process and manipulate documents using various tools.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize Redis queue (optional for CLI)
	var queueAdapter ports.Queue
	if cfg.Redis.Host != "" {
		redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Worker)
		if err != nil {
			log.Printf("⚠️  Redis not available, continuing without queue support: %v", err)
		} else {
			defer redisQueue.Close()
			queueAdapter = adapters.NewQueueAdapter(redisQueue)
		}
	}

	// Initialize processors
	imageProcessor := processors.NewVipsImageProcessor()
	videoProcessor := processors.NewFFmpegVideoProcessor()
	pdfProcessor := processors.NewPlaywrightPDFProcessor(&cfg.External)
	ocrProcessor := processors.NewTesseractOCRProcessor(&cfg.OCR, &cfg.External)
	textExtractor := processors.NewMultiTextExtractor(&cfg.External)

	// Initialize core services (CLI doesn't need all services)
	documentService := services.NewDocumentService(
		nil, // documentRepo
		nil, // jobRepo
		nil, // fileStorage
		queueAdapter,
		imageProcessor,
		videoProcessor,
		pdfProcessor,
		ocrProcessor,
		textExtractor,
		nil, // eventPublisher
	)

	// Initialize health and queue services for CLI
	var healthService ports.HealthService
	var queueService ports.QueueService

	// Always create health service, even without Redis
	healthService = services.NewHealthService(
		queueAdapter, // can be nil
		nil,          // cacheAdapter - not needed for CLI
		nil,          // fileStorage
		imageProcessor,
		videoProcessor,
		pdfProcessor,
		ocrProcessor,
	)

	if queueAdapter != nil {
		queueService = services.NewQueueService(queueAdapter)
	}

	// Initialize CLI adapter
	cliHandler := cli.NewCLI(documentService, healthService, queueService, cfg)

	// Get root command from CLI handler
	rootCmd = cliHandler.GetRootCommand()

	// Add version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Documents Worker CLI v%s", version)
		},
	}
	rootCmd.AddCommand(versionCmd)

	// Execute CLI
	if err := rootCmd.Execute(); err != nil {
		log.Printf("❌ Error: %v", err)
		os.Exit(1)
	}
}
