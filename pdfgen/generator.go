package pdfgen

import (
	"documents-worker/config"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Margins holds PDF margin configuration
type Margins struct {
	Top    string `json:"top"`
	Right  string `json:"right"`
	Bottom string `json:"bottom"`
	Left   string `json:"left"`
}

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
	// startTime := time.Now()

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

// GenerateFromHTMLWithPlaywright creates PDF using Playwright (modern alternative to wkhtmltopdf)
func (pg *PDFGenerator) GenerateFromHTMLWithPlaywright(htmlContent string, options *GenerationOptions) (*GenerationResult, error) {
	// Create temporary HTML file
	htmlFile, err := os.CreateTemp("", "input-*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer os.Remove(htmlFile.Name())

	// Write HTML content
	_, err = htmlFile.WriteString(htmlContent)
	if err != nil {
		htmlFile.Close()
		return nil, fmt.Errorf("failed to write HTML content: %w", err)
	}
	htmlFile.Close()

	return pg.GenerateFromHTMLFileWithPlaywright(htmlFile.Name(), options)
}

// GenerateFromHTMLFileWithPlaywright creates PDF from HTML file using Playwright
func (pg *PDFGenerator) GenerateFromHTMLFileWithPlaywright(htmlPath string, options *GenerationOptions) (*GenerationResult, error) {
	startTime := time.Now()

	// Create output PDF file
	outputFile, err := os.CreateTemp("", "generated-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	outputFile.Close()

	// Build Playwright options
	playwrightOptions := pg.buildPlaywrightOptions(options)

	// Get script path
	scriptPath, err := findPlaywrightScript()
	if err != nil {
		return nil, fmt.Errorf("playwright script not found: %w - run ./scripts/setup-playwright.sh first", err)
	}

	// Execute Playwright script
	cmd := exec.Command("node", scriptPath, htmlPath, outputFile.Name(), playwrightOptions)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("playwright execution failed: %w, output: %s", err, string(output))
	}

	// Parse result from Playwright script
	var playwrightResult struct {
		Success     bool   `json:"success"`
		OutputPath  string `json:"outputPath"`
		FileSize    int64  `json:"fileSize"`
		GeneratedAt string `json:"generatedAt"`
		Error       string `json:"error"`
	}

	if err := parseJSONOutput(string(output), &playwrightResult); err != nil {
		return nil, fmt.Errorf("failed to parse playwright output: %w", err)
	}

	if !playwrightResult.Success {
		return nil, fmt.Errorf("playwright generation failed: %s", playwrightResult.Error)
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
		InputType:   "html",
		GeneratedAt: startTime,
		Duration:    time.Since(startTime),
		FileSize:    fileInfo.Size(),
		PageCount:   pageCount,
		Metadata: map[string]interface{}{
			"generator": "playwright",
			"engine":    "chromium",
		},
	}

	return result, nil
}

// GenerateFromURLWithPlaywright creates PDF from URL using Playwright
func (pg *PDFGenerator) GenerateFromURLWithPlaywright(url string, options *GenerationOptions) (*GenerationResult, error) {
	startTime := time.Now()

	// Create output PDF file
	outputFile, err := os.CreateTemp("", "generated-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	outputFile.Close()

	// Build Playwright options
	playwrightOptions := pg.buildPlaywrightOptions(options)

	// Get script path
	scriptPath, err := findPlaywrightScript()
	if err != nil {
		return nil, fmt.Errorf("playwright script not found: %w - run ./scripts/setup-playwright.sh first", err)
	}

	// Execute Playwright script with URL
	cmd := exec.Command("node", scriptPath, url, outputFile.Name(), playwrightOptions)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("playwright URL generation failed: %w, output: %s", err, string(output))
	}

	// Parse result
	var playwrightResult struct {
		Success     bool   `json:"success"`
		OutputPath  string `json:"outputPath"`
		FileSize    int64  `json:"fileSize"`
		GeneratedAt string `json:"generatedAt"`
		Error       string `json:"error"`
	}

	if err := parseJSONOutput(string(output), &playwrightResult); err != nil {
		return nil, fmt.Errorf("failed to parse playwright output: %w", err)
	}

	if !playwrightResult.Success {
		return nil, fmt.Errorf("playwright URL generation failed: %s", playwrightResult.Error)
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
		InputType:   "url",
		GeneratedAt: startTime,
		Duration:    time.Since(startTime),
		FileSize:    fileInfo.Size(),
		PageCount:   pageCount,
		Metadata: map[string]interface{}{
			"generator":  "playwright",
			"engine":     "chromium",
			"source_url": url,
		},
	}

	return result, nil
}

// buildPlaywrightOptions converts GenerationOptions to JSON string for Playwright script
func (pg *PDFGenerator) buildPlaywrightOptions(options *GenerationOptions) string {
	if options == nil {
		return "{}"
	}

	playwrightOpts := map[string]interface{}{
		"pageSize":    options.PageSize,
		"orientation": options.Orientation,
		"timeout":     30000, // 30 seconds default
		"waitTime":    1000,  // 1 second wait
	}

	// Add margins
	if options.Margins != nil {
		if top, ok := options.Margins["top"]; ok {
			playwrightOpts["marginTop"] = top
		}
		if right, ok := options.Margins["right"]; ok {
			playwrightOpts["marginRight"] = right
		}
		if bottom, ok := options.Margins["bottom"]; ok {
			playwrightOpts["marginBottom"] = bottom
		}
		if left, ok := options.Margins["left"]; ok {
			playwrightOpts["marginLeft"] = left
		}
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(playwrightOpts)
	if err != nil {
		return "{}"
	}

	return string(jsonBytes)
}

// parseJSONOutput parses JSON output from Playwright script
func parseJSONOutput(output string, target interface{}) error {
	// Find the last JSON object in output (in case there are logs before it)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var jsonLine string

	// Look for the last line that looks like JSON
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			jsonLine = line
			break
		}
	}

	if jsonLine == "" {
		return fmt.Errorf("no JSON output found in: %s", output)
	}

	return json.Unmarshal([]byte(jsonLine), target)
}

// findPlaywrightScript searches for the Playwright script starting from current directory
func findPlaywrightScript() (string, error) {
	// Common paths to search
	searchPaths := []string{
		"scripts/playwright/pdf-generator.js",
		"../scripts/playwright/pdf-generator.js",
		"../../scripts/playwright/pdf-generator.js",
		"./scripts/playwright/pdf-generator.js",
	}

	// Get working directory and search from there
	wd, _ := os.Getwd()

	// Try absolute path from working directory
	for _, relPath := range searchPaths {
		fullPath := filepath.Join(wd, relPath)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	// Try to find based on Go module structure
	// Look for go.mod to find project root
	dir := wd
	for i := 0; i < 10; i++ { // Limit search depth
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Found project root
			scriptPath := filepath.Join(dir, "scripts", "playwright", "pdf-generator.js")
			if _, err := os.Stat(scriptPath); err == nil {
				return scriptPath, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached filesystem root
		}
		dir = parent
	}

	return "", fmt.Errorf("playwright script not found in any of the expected locations")
}
