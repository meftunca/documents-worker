package pdfgen

import (
	"documents-worker/config"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PDFGenerator struct {
	config *config.ExternalConfig
}

type GenerationOptions struct {
	PageSize    string            `json:"page_size"`   // A4, Letter, etc.
	Orientation string            `json:"orientation"` // portrait, landscape
	Margins     map[string]string `json:"margins"`     // top, right, bottom, left
	Headers     map[string]string `json:"headers"`     // Custom headers
	Footers     map[string]string `json:"footers"`     // Custom footers
	Metadata    map[string]string `json:"metadata"`    // PDF metadata
	Watermark   string            `json:"watermark"`   // Watermark text
	Quality     int               `json:"quality"`     // Image quality 1-100
}

type GenerationResult struct {
	OutputPath  string                 `json:"output_path"`
	InputType   string                 `json:"input_type"`
	GeneratedAt time.Time              `json:"generated_at"`
	Duration    time.Duration          `json:"duration"`
	FileSize    int64                  `json:"file_size"`
	PageCount   int                    `json:"page_count"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func NewPDFGenerator(externalConfig *config.ExternalConfig) *PDFGenerator {
	return &PDFGenerator{
		config: externalConfig,
	}
}

// GenerateFromHTML creates PDF from HTML content
func (pg *PDFGenerator) GenerateFromHTML(htmlContent string, options *GenerationOptions) (*GenerationResult, error) {
	startTime := time.Now()

	// Create temporary HTML file
	htmlFile, err := os.CreateTemp("", "html-input-*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer os.Remove(htmlFile.Name())

	// Write HTML content
	if _, err := htmlFile.WriteString(htmlContent); err != nil {
		return nil, fmt.Errorf("failed to write HTML content: %w", err)
	}
	htmlFile.Close()

	return pg.GenerateFromHTMLFile(htmlFile.Name(), options)
}

// GenerateFromHTMLFile creates PDF from HTML file
func (pg *PDFGenerator) GenerateFromHTMLFile(htmlPath string, options *GenerationOptions) (*GenerationResult, error) {
	startTime := time.Now()

	// Create output PDF file
	outputFile, err := os.CreateTemp("", "generated-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	outputFile.Close()

	// Build wkhtmltopdf command
	args := pg.buildWkhtmltopdfArgs(htmlPath, outputFile.Name(), options)
	cmd := exec.Command("wkhtmltopdf", args...)

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("wkhtmltopdf execution failed: %w, output: %s", err, string(output))
	}

	// Get file info
	fileInfo, err := os.Stat(outputFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get page count (simplified)
	pageCount, _ := pg.getPDFPageCount(outputFile.Name())

	result := &GenerationResult{
		OutputPath:  outputFile.Name(),
		InputType:   "html",
		GeneratedAt: time.Now(),
		Duration:    time.Since(startTime),
		FileSize:    fileInfo.Size(),
		PageCount:   pageCount,
		Metadata: map[string]interface{}{
			"generator": "wkhtmltopdf",
			"options":   options,
		},
	}

	return result, nil
}

// GenerateFromMarkdown creates PDF from Markdown content
func (pg *PDFGenerator) GenerateFromMarkdown(markdownContent string, options *GenerationOptions) (*GenerationResult, error) {
	startTime := time.Now()

	// Convert Markdown to HTML first
	htmlContent, err := pg.convertMarkdownToHTML(markdownContent)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown to HTML: %w", err)
	}

	// Generate PDF from HTML
	result, err := pg.GenerateFromHTML(htmlContent, options)
	if err != nil {
		return nil, err
	}

	// Update metadata
	result.InputType = "markdown"
	result.Duration = time.Since(startTime)
	result.Metadata["conversion_step"] = "markdown_to_html_to_pdf"

	return result, nil
}

// GenerateFromOfficeDocument creates PDF from Office documents
func (pg *PDFGenerator) GenerateFromOfficeDocument(docPath string, options *GenerationOptions) (*GenerationResult, error) {
	startTime := time.Now()

	// Use LibreOffice for conversion
	outputFile, err := os.CreateTemp("", "office-generated-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	outputFile.Close()

	// Convert using LibreOffice
	outputDir := filepath.Dir(outputFile.Name())
	cmd := exec.Command(pg.config.LibreOfficePath,
		"--headless",
		"--convert-to", "pdf",
		"--outdir", outputDir,
		docPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("libreoffice conversion failed: %w, output: %s", err, string(output))
	}

	// LibreOffice creates file with original name + .pdf
	originalName := strings.TrimSuffix(filepath.Base(docPath), filepath.Ext(docPath))
	libreOfficePDF := filepath.Join(outputDir, originalName+".pdf")

	// Move to our expected location
	if err := os.Rename(libreOfficePDF, outputFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to move generated PDF: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(outputFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get page count
	pageCount, _ := pg.getPDFPageCount(outputFile.Name())

	result := &GenerationResult{
		OutputPath:  outputFile.Name(),
		InputType:   "office_document",
		GeneratedAt: time.Now(),
		Duration:    time.Since(startTime),
		FileSize:    fileInfo.Size(),
		PageCount:   pageCount,
		Metadata: map[string]interface{}{
			"generator":   "libreoffice",
			"source_file": filepath.Base(docPath),
			"source_type": filepath.Ext(docPath),
		},
	}

	return result, nil
}

// buildWkhtmltopdfArgs builds command arguments for wkhtmltopdf
func (pg *PDFGenerator) buildWkhtmltopdfArgs(inputPath, outputPath string, options *GenerationOptions) []string {
	args := []string{}

	if options == nil {
		options = &GenerationOptions{
			PageSize:    "A4",
			Orientation: "portrait",
			Quality:     90,
		}
	}

	// Page size
	if options.PageSize != "" {
		args = append(args, "--page-size", options.PageSize)
	}

	// Orientation
	if options.Orientation != "" {
		args = append(args, "--orientation", options.Orientation)
	}

	// Margins
	if options.Margins != nil {
		if top, ok := options.Margins["top"]; ok {
			args = append(args, "--margin-top", top)
		}
		if right, ok := options.Margins["right"]; ok {
			args = append(args, "--margin-right", right)
		}
		if bottom, ok := options.Margins["bottom"]; ok {
			args = append(args, "--margin-bottom", bottom)
		}
		if left, ok := options.Margins["left"]; ok {
			args = append(args, "--margin-left", left)
		}
	}

	// Quality
	if options.Quality > 0 {
		args = append(args, "--image-quality", fmt.Sprintf("%d", options.Quality))
	}

	// Headers
	if options.Headers != nil {
		if center, ok := options.Headers["center"]; ok {
			args = append(args, "--header-center", center)
		}
		if left, ok := options.Headers["left"]; ok {
			args = append(args, "--header-left", left)
		}
		if right, ok := options.Headers["right"]; ok {
			args = append(args, "--header-right", right)
		}
	}

	// Footers
	if options.Footers != nil {
		if center, ok := options.Footers["center"]; ok {
			args = append(args, "--footer-center", center)
		}
		if left, ok := options.Footers["left"]; ok {
			args = append(args, "--footer-left", left)
		}
		if right, ok := options.Footers["right"]; ok {
			args = append(args, "--footer-right", right)
		}
	}

	// Enable local file access
	args = append(args, "--enable-local-file-access")

	// Input and output
	args = append(args, inputPath, outputPath)

	return args
}

// convertMarkdownToHTML converts markdown to HTML using a markdown processor
func (pg *PDFGenerator) convertMarkdownToHTML(markdownContent string) (string, error) {
	// Create temporary markdown file
	mdFile, err := os.CreateTemp("", "markdown-input-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp markdown file: %w", err)
	}
	defer os.Remove(mdFile.Name())

	// Write markdown content
	if _, err := mdFile.WriteString(markdownContent); err != nil {
		return "", fmt.Errorf("failed to write markdown content: %w", err)
	}
	mdFile.Close()

	// Create temporary HTML file
	htmlFile, err := os.CreateTemp("", "markdown-output-*.html")
	if err != nil {
		return "", fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer os.Remove(htmlFile.Name())
	htmlFile.Close()

	// Convert using pandoc
	cmd := exec.Command("pandoc",
		"-f", "markdown",
		"-t", "html5",
		"--standalone",
		"--css", pg.getDefaultCSS(),
		"-o", htmlFile.Name(),
		mdFile.Name(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pandoc conversion failed: %w, output: %s", err, string(output))
	}

	// Read converted HTML
	htmlBytes, err := os.ReadFile(htmlFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read converted HTML: %w", err)
	}

	return string(htmlBytes), nil
}

// getPDFPageCount gets the number of pages in a PDF
func (pg *PDFGenerator) getPDFPageCount(pdfPath string) (int, error) {
	cmd := exec.Command(pg.config.MutoolPath, "info", pdfPath)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Pages:") {
			var pageCount int
			if _, err := fmt.Sscanf(line, "Pages: %d", &pageCount); err == nil {
				return pageCount, nil
			}
		}
	}

	return 1, nil // Default to 1 if can't determine
}

// getDefaultCSS returns default CSS for markdown conversion
func (pg *PDFGenerator) getDefaultCSS() string {
	return `
body {
    font-family: Arial, sans-serif;
    line-height: 1.6;
    max-width: 800px;
    margin: 0 auto;
    padding: 20px;
    color: #333;
}

h1, h2, h3, h4, h5, h6 {
    color: #2c3e50;
    margin-top: 24px;
    margin-bottom: 16px;
}

h1 {
    border-bottom: 2px solid #eee;
    padding-bottom: 10px;
}

pre {
    background: #f8f8f8;
    border: 1px solid #ddd;
    border-radius: 4px;
    padding: 12px;
    overflow-x: auto;
}

code {
    background: #f8f8f8;
    padding: 2px 4px;
    border-radius: 3px;
    font-family: Consolas, Monaco, monospace;
}

blockquote {
    border-left: 4px solid #ddd;
    margin: 0;
    padding-left: 16px;
    color: #666;
}

table {
    border-collapse: collapse;
    width: 100%;
    margin: 16px 0;
}

th, td {
    border: 1px solid #ddd;
    padding: 8px 12px;
    text-align: left;
}

th {
    background-color: #f8f8f8;
    font-weight: bold;
}
`
}
