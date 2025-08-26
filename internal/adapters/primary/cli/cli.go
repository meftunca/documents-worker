package cli

import (
	"bytes"
	"context"
	"documents-worker/chunking"
	"documents-worker/config"
	"documents-worker/internal/core/domain"
	"documents-worker/internal/core/ports"
	"documents-worker/pdfgen"
	"documents-worker/utils"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// CLI represents the command line interface
type CLI struct {
	documentService ports.DocumentService
	healthService   ports.HealthService
	queueService    ports.QueueService
	config          *config.Config
}

// NewCLI creates a new CLI instance
func NewCLI(
	documentService ports.DocumentService,
	healthService ports.HealthService,
	queueService ports.QueueService,
	config *config.Config,
) *CLI {
	return &CLI{
		documentService: documentService,
		healthService:   healthService,
		queueService:    queueService,
		config:          config,
	}
}

// GetRootCommand returns the root cobra command
func (cli *CLI) GetRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "documents-worker",
		Short: "Documents Worker CLI - Process documents via command line",
		Long: `Documents Worker CLI provides command line access to document processing capabilities.
		
Features:
- Convert images between formats (JPEG, PNG, WEBP, AVIF)
- Generate PDF from HTML
- Extract text from documents
- Perform OCR on images and PDFs
- Generate video thumbnails
- Process documents in batch`,
		Version: "1.0.0",
	}

	// Add subcommands
	rootCmd.AddCommand(cli.getConvertCommand())
	rootCmd.AddCommand(cli.getOCRCommand())
	rootCmd.AddCommand(cli.getExtractCommand())
	rootCmd.AddCommand(cli.getThumbnailCommand())
	rootCmd.AddCommand(cli.getHealthCommand())
	rootCmd.AddCommand(cli.getStatsCommand())

	return rootCmd
}

// getConvertCommand returns the convert command
func (cli *CLI) getConvertCommand() *cobra.Command {
	convertCmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert documents between formats",
		Long:  "Convert images, videos, and documents between different formats",
	}

	// Image conversion
	imageCmd := &cobra.Command{
		Use:   "image [input] [output] [format]",
		Short: "Convert image to different format",
		Long:  "Convert image files between JPEG, PNG, WEBP, AVIF formats",
		Args:  cobra.ExactArgs(3),
		RunE:  cli.convertImage,
	}
	imageCmd.Flags().Int("width", 0, "Output width (0 = maintain aspect ratio)")
	imageCmd.Flags().Int("height", 0, "Output height (0 = maintain aspect ratio)")
	imageCmd.Flags().Int("quality", 85, "Output quality (1-100)")

	// PDF generation
	pdfCmd := &cobra.Command{
		Use:   "pdf [input] [output]",
		Short: "Generate PDF from HTML",
		Long:  "Generate PDF from HTML file or URL",
		Args:  cobra.ExactArgs(2),
		RunE:  cli.generatePDF,
	}
	pdfCmd.Flags().String("page-size", "A4", "Page size (A4, A3, Letter, etc.)")
	pdfCmd.Flags().String("orientation", "portrait", "Page orientation (portrait, landscape)")
	pdfCmd.Flags().Bool("url", false, "Input is a URL instead of file")

	// Document chunking
	chunkCmd := &cobra.Command{
		Use:   "chunk [input] [output_dir]",
		Short: "Split documents into smaller chunks",
		Long: `Split documents into smaller, manageable chunks for easier processing and sharing.
		
Supported chunking methods:
- Text-based: Split by paragraphs, sentences, or character count
- PDF: Split by pages or page ranges  
- Office: Split by slides (PowerPoint), sheets (Excel), or pages (Word)
- Smart: Intelligent content-aware splitting`,
		Args: cobra.ExactArgs(2),
		RunE: cli.chunkDocument,
	}
	chunkCmd.Flags().String("method", "smart", "Chunking method (text, semantic, recursive, smart)")
	chunkCmd.Flags().Int("size", 256, "Chunk size in characters (for text-based methods)")
	chunkCmd.Flags().Int("overlap", 20, "Overlap between chunks in characters")
	chunkCmd.Flags().Int("pages-per-chunk", 5, "Pages per chunk (for pages method)")
	chunkCmd.Flags().String("format", "auto", "Output format (txt, md, pdf, auto)")
	chunkCmd.Flags().Bool("preserve-formatting", true, "Preserve original formatting")

	convertCmd.AddCommand(imageCmd)
	convertCmd.AddCommand(pdfCmd)
	convertCmd.AddCommand(chunkCmd)

	return convertCmd
}

// getOCRCommand returns the OCR command
func (cli *CLI) getOCRCommand() *cobra.Command {
	ocrCmd := &cobra.Command{
		Use:   "ocr [input] [output]",
		Short: "Perform OCR on images or PDFs",
		Long:  "Extract text from images or PDF files using OCR",
		Args:  cobra.ExactArgs(2),
		RunE:  cli.performOCR,
	}
	ocrCmd.Flags().String("lang", "eng", "OCR language (eng, tur, fra, etc.)")

	return ocrCmd
}

// getExtractCommand returns the extract command
func (cli *CLI) getExtractCommand() *cobra.Command {
	extractCmd := &cobra.Command{
		Use:   "extract [input] [output]",
		Short: "Extract text from documents",
		Long:  "Extract text from PDF, Office documents, or text files",
		Args:  cobra.ExactArgs(2),
		RunE:  cli.extractText,
	}

	return extractCmd
}

// getThumbnailCommand returns the thumbnail command
func (cli *CLI) getThumbnailCommand() *cobra.Command {
	thumbnailCmd := &cobra.Command{
		Use:   "thumbnail [input] [output]",
		Short: "Generate thumbnails from images or videos",
		Long:  "Generate thumbnail images from image or video files",
		Args:  cobra.ExactArgs(2),
		RunE:  cli.generateThumbnail,
	}
	thumbnailCmd.Flags().Int("size", 200, "Thumbnail size (width/height)")
	thumbnailCmd.Flags().Int("time", 0, "Time offset for video thumbnail (seconds)")

	return thumbnailCmd
}

// getHealthCommand returns the health command
func (cli *CLI) getHealthCommand() *cobra.Command {
	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "Check system health",
		Long:  "Check the health status of the document processing system",
		RunE:  cli.checkHealth,
	}

	return healthCmd
}

// getStatsCommand returns the stats command
func (cli *CLI) getStatsCommand() *cobra.Command {
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show system statistics",
		Long:  "Show queue statistics and system information",
		RunE:  cli.showStats,
	}

	return statsCmd
}

// convertImage handles image conversion
func (cli *CLI) convertImage(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outputPath := args[1]
	outputFormat := args[2]

	// Get flags
	width, _ := cmd.Flags().GetInt("width")
	height, _ := cmd.Flags().GetInt("height")
	quality, _ := cmd.Flags().GetInt("quality")

	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// Prepare parameters
	params := map[string]interface{}{
		"quality": quality,
	}
	if width > 0 {
		params["width"] = width
	}
	if height > 0 {
		params["height"] = height
	}

	// Convert image
	fmt.Printf("Converting %s to %s format...\n", inputPath, outputFormat)
	result, err := cli.documentService.ConvertImage(context.Background(), inputFile, outputFormat, params)
	if err != nil {
		return fmt.Errorf("failed to convert image: %w", err)
	}

	// Save output
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, result)
	if err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	fmt.Printf("✅ Image converted successfully: %s\n", outputPath)
	return nil
}

// generatePDF handles PDF generation from various formats
func (cli *CLI) generatePDF(cmd *cobra.Command, args []string) error {
	input := args[0]
	outputPath := args[1]

	// Get flags
	pageSize, _ := cmd.Flags().GetString("page-size")
	orientation, _ := cmd.Flags().GetString("orientation")
	isURL, _ := cmd.Flags().GetBool("url")

	// Prepare parameters
	params := map[string]interface{}{
		"page_size":   pageSize,
		"orientation": orientation,
	}

	var result io.Reader
	var err error

	if isURL {
		fmt.Printf("Generating PDF from URL: %s...\n", input)
		// For URL, we would need a different method
		return fmt.Errorf("URL to PDF conversion not implemented in this example")
	} else {
		// Determine input file type by extension first
		ext := strings.ToLower(filepath.Ext(input))
		var fileType string

		if ext != "" {
			// If extension exists, use it
			fileType = cli.getFileTypeFromExtension(ext)
		} else {
			// No extension, detect MIME type
			fmt.Printf("No file extension detected, analyzing content...\n")
			mimeType, err := utils.DetectMimeTypeFromFile(input)
			if err != nil {
				return fmt.Errorf("failed to detect file type: %w", err)
			}
			fileType = cli.getFileTypeFromMimeType(mimeType)
			fmt.Printf("Detected content type: %s -> %s\n", mimeType, fileType)
		}

		switch fileType {
		case "html":
			fmt.Printf("Generating PDF from HTML file: %s...\n", input)
			result, err = cli.generatePDFFromHTML(input, params)

		case "markdown":
			fmt.Printf("Generating PDF from Markdown file: %s...\n", input)
			result, err = cli.generatePDFFromMarkdown(input, params)

		case "office":
			fmt.Printf("Generating PDF from Office document: %s...\n", input)
			result, err = cli.generatePDFFromOffice(input, params)

		default:
			if ext != "" {
				return fmt.Errorf("unsupported file format: %s. Supported formats: HTML (.html, .htm), Markdown (.md, .markdown), Office documents (.docx, .xlsx, .pptx, etc.)", ext)
			} else {
				return fmt.Errorf("unsupported file content type. Supported formats: HTML, Markdown, Office documents")
			}
		}

		if err != nil {
			return fmt.Errorf("failed to generate PDF: %w", err)
		}
	}

	// Save output
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, result)
	if err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	fmt.Printf("✅ PDF generated successfully: %s\n", outputPath)
	return nil
}

// performOCR handles OCR processing
func (cli *CLI) performOCR(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outputPath := args[1]

	// Get flags
	language, _ := cmd.Flags().GetString("lang")

	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	fmt.Printf("Performing OCR on %s (language: %s)...\n", inputPath, language)
	text, err := cli.documentService.PerformOCR(context.Background(), inputFile, language)
	if err != nil {
		return fmt.Errorf("failed to perform OCR: %w", err)
	}

	// Save output
	err = os.WriteFile(outputPath, []byte(text), 0644)
	if err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	fmt.Printf("✅ OCR completed successfully: %s\n", outputPath)
	fmt.Printf("📄 Extracted %d characters\n", len(text))
	return nil
}

// extractText handles text extraction
func (cli *CLI) extractText(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outputPath := args[1]

	// Determine document type from extension
	ext := strings.ToLower(filepath.Ext(inputPath))
	var docType domain.DocumentType

	switch ext {
	case ".pdf":
		docType = domain.DocumentTypePDF
	case ".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt":
		docType = domain.DocumentTypeOffice
	case ".txt", ".md":
		docType = domain.DocumentTypeText
	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	fmt.Printf("Extracting text from %s...\n", inputPath)
	text, err := cli.documentService.ExtractText(context.Background(), inputFile, docType)
	if err != nil {
		return fmt.Errorf("failed to extract text: %w", err)
	}

	// Save output
	err = os.WriteFile(outputPath, []byte(text), 0644)
	if err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	fmt.Printf("✅ Text extracted successfully: %s\n", outputPath)
	fmt.Printf("📄 Extracted %d characters\n", len(text))
	return nil
}

// generateThumbnail handles thumbnail generation
func (cli *CLI) generateThumbnail(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outputPath := args[1]

	// Get flags
	size, _ := cmd.Flags().GetInt("size")
	timeOffset, _ := cmd.Flags().GetInt("time")

	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// Prepare parameters
	params := map[string]interface{}{
		"size": size,
	}
	if timeOffset > 0 {
		params["time_offset"] = timeOffset
	}

	fmt.Printf("Generating thumbnail from %s (size: %dx%d)...\n", inputPath, size, size)
	result, err := cli.documentService.GenerateThumbnail(context.Background(), inputFile, params)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	// Save output
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, result)
	if err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	fmt.Printf("✅ Thumbnail generated successfully: %s\n", outputPath)
	return nil
}

// checkHealth handles health check
func (cli *CLI) checkHealth(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Checking system health...")

	// Create timeout context for health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := cli.healthService.GetHealthStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get health status: %w", err)
	}

	// Pretty print health status
	healthJSON, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format health status: %w", err)
	}

	fmt.Printf("\n%s\n", string(healthJSON))

	if health.Status == "healthy" {
		fmt.Printf("\n✅ System is healthy\n")
	} else {
		fmt.Printf("\n⚠️  System status: %s\n", health.Status)
	}

	return nil
}

// showStats handles statistics display
func (cli *CLI) showStats(cmd *cobra.Command, args []string) error {
	fmt.Println("📊 Getting queue statistics...")

	stats, err := cli.queueService.GetQueueStats(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get queue stats: %w", err)
	}

	// Pretty print stats
	fmt.Printf("\n📈 Queue Statistics:\n")
	fmt.Printf("  Pending Jobs:    %d\n", stats.PendingJobs)
	fmt.Printf("  Processing Jobs: %d\n", stats.ProcessingJobs)
	fmt.Printf("  Completed Jobs:  %d\n", stats.CompletedJobs)
	fmt.Printf("  Failed Jobs:     %d\n", stats.FailedJobs)
	fmt.Printf("  Total Jobs:      %d\n", stats.TotalJobs)
	fmt.Printf("  Timestamp:       %s\n", stats.Timestamp.Format(time.RFC3339))

	return nil
}

// Helper functions for PDF generation

// generatePDFFromHTML generates PDF from HTML file
func (cli *CLI) generatePDFFromHTML(input string, params map[string]interface{}) (io.Reader, error) {
	inputFile, err := os.Open(input)
	if err != nil {
		return nil, fmt.Errorf("failed to open HTML file: %w", err)
	}
	defer inputFile.Close()

	return cli.documentService.GeneratePDF(context.Background(), inputFile, params)
}

// generatePDFFromMarkdown generates PDF from Markdown file
func (cli *CLI) generatePDFFromMarkdown(input string, params map[string]interface{}) (io.Reader, error) {
	// Read markdown content
	content, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read markdown file: %w", err)
	}

	// Convert markdown to HTML with basic markdown parsing
	htmlContent := cli.convertMarkdownToHTML(string(content))

	// Create a temp HTML file and use the HTML to PDF conversion
	htmlReader := strings.NewReader(htmlContent)
	return cli.documentService.GeneratePDF(context.Background(), htmlReader, params)
}

// convertMarkdownToHTML performs markdown to HTML conversion using Goldmark
func (cli *CLI) convertMarkdownToHTML(markdown string) string {
	// Configure goldmark with extensions for better markdown support
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,           // GitHub Flavored Markdown
			extension.Table,         // Tables
			extension.Strikethrough, // ~~strikethrough~~
			extension.TaskList,      // - [x] task lists
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // Hard line breaks
			html.WithXHTML(),     // XHTML compatible
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		// Fallback to original content if conversion fails
		return fmt.Sprintf("<pre>%s</pre>", markdown)
	}

	// Wrap in proper HTML structure with beautiful styling
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Converted Markdown</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; 
            margin: 40px auto; 
            line-height: 1.6; 
            color: #24292e;
            max-width: 900px;
            background-color: #fff;
        }
        
        h1, h2, h3, h4, h5, h6 { 
            color: #24292e; 
            margin-top: 24px; 
            margin-bottom: 16px; 
            font-weight: 600;
            line-height: 1.25;
        }
        
        h1 { 
            font-size: 2em;
            border-bottom: 1px solid #eaecef; 
            padding-bottom: 10px; 
        }
        
        h2 { 
            font-size: 1.5em;
            border-bottom: 1px solid #eaecef; 
            padding-bottom: 8px; 
        }
        
        h3 { font-size: 1.25em; }
        h4 { font-size: 1em; }
        h5 { font-size: 0.875em; }
        h6 { font-size: 0.85em; color: #6a737d; }
        
        p { 
            margin-bottom: 16px; 
            margin-top: 0;
        }
        
        strong, b { 
            font-weight: 600; 
        }
        
        em, i { 
            font-style: italic; 
        }
        
        hr { 
            border: none; 
            border-top: 1px solid #e1e4e8; 
            margin: 24px 0; 
            height: 0.25em;
            background-color: #e1e4e8;
        }
        
        pre { 
            background: #f6f8fa; 
            border: 1px solid #e1e4e8; 
            border-radius: 6px; 
            padding: 16px; 
            overflow-x: auto; 
            font-family: 'SFMono-Regular', 'Consolas', 'Liberation Mono', 'Menlo', monospace;
            font-size: 85%%;
        }
        
        code { 
            background: #f6f8fa; 
            padding: 2px 4px; 
            border-radius: 3px; 
            font-family: 'SFMono-Regular', 'Consolas', 'Liberation Mono', 'Menlo', monospace; 
            font-size: 85%%;
        }
        
        pre code {
            background: transparent;
            padding: 0;
        }
        
        ul, ol { 
            margin-left: 0; 
            margin-bottom: 16px; 
            padding-left: 30px;
        }
        
        li { 
            margin-bottom: 4px; 
        }
        
        blockquote {
            border-left: 4px solid #dfe2e5;
            margin: 0 0 16px 0;
            padding: 0 16px;
            color: #6a737d;
        }
        
        table {
            border-collapse: collapse;
            margin-bottom: 16px;
            width: 100%%;
        }
        
        table th, table td {
            border: 1px solid #dfe2e5;
            padding: 6px 13px;
        }
        
        table th {
            background-color: #f6f8fa;
            font-weight: 600;
        }
        
        .task-list-item {
            list-style-type: none;
        }
        
        .task-list-item input {
            margin-right: 4px;
        }
        
        del {
            text-decoration: line-through;
            opacity: 0.7;
        }
        
        a {
            color: #0366d6;
            text-decoration: none;
        }
        
        a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    %s
</body>
</html>`, buf.String())
}

// generatePDFFromOffice generates PDF from Office documents
func (cli *CLI) generatePDFFromOffice(input string, params map[string]interface{}) (io.Reader, error) {
	// Create a PDF generator instance
	pdfGenerator := pdfgen.NewPDFGenerator(&cli.config.External)

	// Convert office document to PDF using LibreOffice
	options := &pdfgen.GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
	}

	// Override with params if provided
	if pageSize, ok := params["page_size"].(string); ok {
		options.PageSize = pageSize
	}
	if orientation, ok := params["orientation"].(string); ok {
		options.Orientation = orientation
	}

	// Generate PDF from office document
	result, err := pdfGenerator.GenerateFromOfficeDocument(input, options)
	if err != nil {
		return nil, fmt.Errorf("office document to PDF conversion failed: %w", err)
	}

	// Read the generated PDF file
	pdfFile, err := os.Open(result.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open generated PDF: %w", err)
	}

	// Note: The caller is responsible for closing the file
	// We should clean up the temp file after reading
	go func() {
		defer os.Remove(result.OutputPath)
	}()

	return pdfFile, nil
}

// getFileTypeFromExtension determines file type from extension
func (cli *CLI) getFileTypeFromExtension(ext string) string {
	switch ext {
	case ".html", ".htm":
		return "html"
	case ".md", ".markdown":
		return "markdown"
	case ".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt", ".odt", ".ods", ".odp":
		return "office"
	default:
		return "unknown"
	}
}

// getFileTypeFromMimeType determines file type from MIME type
func (cli *CLI) getFileTypeFromMimeType(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "text/html"):
		return "html"
	case strings.Contains(mimeType, "text/markdown") || strings.Contains(mimeType, "text/x-markdown"):
		return "markdown"
	case strings.Contains(mimeType, "text/plain"):
		// For plain text, try to detect if it's markdown by checking content patterns
		return "markdown" // Assume plain text might be markdown
	case utils.IsOfficeDocument(mimeType):
		return "office"
	case strings.Contains(mimeType, "application/vnd.openxmlformats-officedocument"):
		return "office"
	case strings.Contains(mimeType, "application/vnd.ms-"):
		return "office"
	case strings.Contains(mimeType, "application/vnd.oasis.opendocument"):
		return "office"
	default:
		return "unknown"
	}
}

// chunkDocument handles document chunking using modern chunking service
func (cli *CLI) chunkDocument(cmd *cobra.Command, args []string) error {
	input := args[0]
	outputDir := args[1]

	// Get flags
	method, _ := cmd.Flags().GetString("method")
	chunkSize, _ := cmd.Flags().GetInt("size")
	overlap, _ := cmd.Flags().GetInt("overlap")
	outputFormat, _ := cmd.Flags().GetString("format")
	preserveFormatting, _ := cmd.Flags().GetBool("preserve-formatting")

	fmt.Printf("🔄 Chunking document: %s\n", input)
	fmt.Printf("📐 Method: %s, Chunk size: %d chars, Overlap: %d chars\n", method, chunkSize, overlap)
	fmt.Printf("📁 Output directory: %s\n", outputDir)

	// Create chunking service
	chunkingService := chunking.NewService()

	// Create chunking config
	config := chunking.ChunkConfig{
		Method:             chunking.ChunkMethod(method),
		ChunkSize:          chunkSize,
		Overlap:            overlap,
		OutputFormat:       outputFormat,
		PreserveFormatting: preserveFormatting,
	}

	// Chunk the document
	result, err := chunkingService.ChunkFromFile(context.Background(), input, config)
	if err != nil {
		return fmt.Errorf("failed to chunk document: %w", err)
	}

	// Save chunks
	if err := chunkingService.SaveChunks(context.Background(), result, outputDir); err != nil {
		return fmt.Errorf("failed to save chunks: %w", err)
	}

	fmt.Printf("✅ Successfully created %d chunks in %s\n", result.TotalChunks, outputDir)
	fmt.Printf("📊 Average chunk size: %.0f characters\n", result.AverageSize)
	fmt.Printf("📈 Compression ratio: %.1f%% (original: %d chars)\n",
		float64(result.TotalChunks)*result.AverageSize/float64(result.OriginalSize)*100,
		result.OriginalSize)

	return nil
}
