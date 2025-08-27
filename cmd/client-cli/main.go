package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"documents-worker/cmd/client"
)

func main() {
	var (
		baseURL     = flag.String("url", "http://localhost:8080", "Documents worker service URL")
		apiKey      = flag.String("key", "", "API key for authentication")
		operation   = flag.String("op", "", "Operation: document, image, video, text, ocr, chunk")
		filePath    = flag.String("file", "", "Path to the file to process")
		batch       = flag.String("batch", "", "Directory path for batch processing")
		format      = flag.String("format", "", "Output format")
		quality     = flag.Int("quality", 0, "Quality (1-100)")
		width       = flag.Int("width", 0, "Width in pixels")
		height      = flag.Int("height", 0, "Height in pixels")
		page        = flag.Int("page", 0, "Page number for PDF processing")
		language    = flag.String("lang", "tur", "Language for OCR")
		wait        = flag.Bool("wait", false, "Wait for job completion")
		jobID       = flag.String("job", "", "Job ID to check status")
		download    = flag.Bool("download", false, "Download job result")
		outputDir   = flag.String("output", ".", "Output directory for results")
		concurrent  = flag.Int("concurrent", 5, "Max concurrent jobs for batch processing")
		timeout     = flag.Duration("timeout", 30*time.Second, "Request timeout")
		verbose     = flag.Bool("v", false, "Verbose output")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Documents Worker Client\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Operations:\n")
		fmt.Fprintf(os.Stderr, "  document  - Process office documents\n")
		fmt.Fprintf(os.Stderr, "  image     - Process images\n")
		fmt.Fprintf(os.Stderr, "  video     - Process videos\n")
		fmt.Fprintf(os.Stderr, "  text      - Extract text\n")
		fmt.Fprintf(os.Stderr, "  ocr       - Perform OCR\n")
		fmt.Fprintf(os.Stderr, "  chunk     - Chunk documents for RAG\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s -op=image -file=photo.jpg -format=webp -quality=85\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -op=document -file=doc.pdf -wait\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -job=abc123 -download -output=./results\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -op=text -batch=./docs -concurrent=10\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Create client
	clientConfig := client.Config{
		BaseURL: *baseURL,
		APIKey:  *apiKey,
		Timeout: *timeout,
	}

	c := client.NewClient(clientConfig)

	// Handle different modes
	switch {
	case *jobID != "":
		if err := handleJobStatus(c, *jobID, *download, *outputDir, *verbose); err != nil {
			log.Fatalf("Job operation failed: %v", err)
		}

	case *batch != "":
		if *operation == "" {
			log.Fatal("Operation is required for batch processing")
		}
		if err := handleBatchProcessing(c, *batch, *operation, createOptions(*format, *quality, *width, *height, *page, *language), *concurrent, *wait, *verbose); err != nil {
			log.Fatalf("Batch processing failed: %v", err)
		}

	case *filePath != "":
		if *operation == "" {
			log.Fatal("Operation is required")
		}
		if err := handleSingleFile(c, *filePath, *operation, createOptions(*format, *quality, *width, *height, *page, *language), *wait, *verbose); err != nil {
			log.Fatalf("File processing failed: %v", err)
		}

	default:
		// Health check
		if err := handleHealthCheck(c, *verbose); err != nil {
			log.Fatalf("Health check failed: %v", err)
		}
	}
}

func createOptions(format string, quality, width, height, page int, language string) *client.ProcessingOptions {
	options := &client.ProcessingOptions{
		Metadata: make(map[string]string),
	}

	if format != "" {
		options.Format = format
	}
	if quality > 0 {
		options.Quality = quality
	}
	if width > 0 {
		options.Width = width
	}
	if height > 0 {
		options.Height = height
	}
	if page > 0 {
		options.Page = page
	}
	if language != "" {
		options.Language = language
	}

	return options
}

func handleHealthCheck(c *client.Client, verbose bool) error {
	if verbose {
		fmt.Println("🏥 Checking service health...")
	}

	resp, err := c.Health()
	if err != nil {
		return err
	}

	if resp.Success {
		fmt.Println("✅ Service is healthy")
	} else {
		fmt.Printf("❌ Service health check failed: %s\n", resp.Message)
	}

	if verbose {
		fmt.Printf("Response: %+v\n", resp)
	}

	return nil
}

func handleSingleFile(c *client.Client, filePath, operation string, options *client.ProcessingOptions, wait, verbose bool) error {
	if verbose {
		fmt.Printf("📄 Processing file: %s\n", filePath)
		fmt.Printf("🔧 Operation: %s\n", operation)
	}

	if wait {
		status, err := c.ProcessAndWait(filePath, operation, options)
		if err != nil {
			return err
		}

		fmt.Printf("✅ Job completed: %s\n", status.ID)
		if verbose {
			fmt.Printf("Status: %+v\n", status)
		}

		return nil
	}

	// Submit job only
	var resp *client.Response
	var err error

	switch operation {
	case "document":
		resp, err = c.ProcessDocument(filePath, options)
	case "image":
		resp, err = c.ProcessImage(filePath, options)
	case "video":
		resp, err = c.ProcessVideo(filePath, options)
	case "text":
		resp, err = c.ExtractText(filePath, options)
	case "ocr":
		resp, err = c.PerformOCR(filePath, options)
	case "chunk":
		resp, err = c.ChunkDocument(filePath, options)
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}

	if err != nil {
		return err
	}

	fmt.Printf("📤 Job submitted: %s\n", resp.JobID)
	if verbose {
		fmt.Printf("Response: %+v\n", resp)
	}

	return nil
}

func handleBatchProcessing(c *client.Client, batchDir, operation string, options *client.ProcessingOptions, concurrent int, wait, verbose bool) error {
	if verbose {
		fmt.Printf("📁 Batch processing directory: %s\n", batchDir)
		fmt.Printf("🔧 Operation: %s\n", operation)
		fmt.Printf("⚡ Concurrent jobs: %d\n", concurrent)
	}

	// Find files
	files, err := findFiles(batchDir)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Println("⚠️  No files found in directory")
		return nil
	}

	fmt.Printf("📊 Found %d files to process\n", len(files))

	if wait {
		statuses, err := c.BatchProcess(files, operation, options, concurrent)
		if err != nil {
			return err
		}

		fmt.Printf("✅ Batch processing completed: %d jobs\n", len(statuses))
		
		if verbose {
			for _, status := range statuses {
				fmt.Printf("Job %s: %s\n", status.ID, status.Status)
			}
		}

		return nil
	}

	// Submit jobs only (simplified)
	fmt.Println("📤 Submitting jobs (use -wait to wait for completion)")
	for i, file := range files {
		if i >= concurrent {
			break // Limit initial submissions
		}
		
		var resp *client.Response
		switch operation {
		case "document":
			resp, err = c.ProcessDocument(file, options)
		case "image":
			resp, err = c.ProcessImage(file, options)
		case "video":
			resp, err = c.ProcessVideo(file, options)
		case "text":
			resp, err = c.ExtractText(file, options)
		case "ocr":
			resp, err = c.PerformOCR(file, options)
		case "chunk":
			resp, err = c.ChunkDocument(file, options)
		}

		if err != nil {
			fmt.Printf("❌ Failed to submit %s: %v\n", file, err)
			continue
		}

		fmt.Printf("✓ Submitted %s -> %s\n", filepath.Base(file), resp.JobID)
	}

	return nil
}

func handleJobStatus(c *client.Client, jobID string, download bool, outputDir string, verbose bool) error {
	if verbose {
		fmt.Printf("🔍 Checking job status: %s\n", jobID)
	}

	status, err := c.GetJobStatus(jobID)
	if err != nil {
		return err
	}

	fmt.Printf("📊 Job %s: %s\n", jobID, status.Status)
	if status.Progress > 0 {
		fmt.Printf("📈 Progress: %d%%\n", status.Progress)
	}

	if verbose {
		fmt.Printf("Status: %+v\n", status)
	}

	if download && (status.Status == "completed" || status.Status == "success") {
		if verbose {
			fmt.Printf("📥 Downloading result to: %s\n", outputDir)
		}

		result, err := c.GetJobResult(jobID)
		if err != nil {
			return err
		}
		defer result.Close()

		// Save result
		outputPath := filepath.Join(outputDir, fmt.Sprintf("result_%s", jobID))
		outputFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer outputFile.Close()

		if _, err := outputFile.ReadFrom(result); err != nil {
			return err
		}

		fmt.Printf("💾 Result saved to: %s\n", outputPath)
	}

	return nil
}

func findFiles(dir string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Filter by common file extensions
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".pdf", ".doc", ".docx", ".txt", ".md", ".html", ".htm",
			 ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp", ".avif",
			 ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm":
			files = append(files, path)
		}

		return nil
	})

	return files, err
}
