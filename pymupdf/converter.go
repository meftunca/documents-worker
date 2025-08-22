package pymupdf

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ConversionResult represents the result of a PyMuPDF conversion
type ConversionResult struct {
	Success        bool                   `json:"success"`
	InputPath      string                 `json:"input_path"`
	OutputPath     string                 `json:"output_path"`
	ConversionType string                 `json:"conversion_type"`
	PagesProcessed int                    `json:"pages_processed,omitempty"`
	Duration       float64                `json:"duration"`
	FileSize       int64                  `json:"file_size"`
	WordCount      int                    `json:"word_count"`
	CharCount      int                    `json:"char_count"`
	Metadata       map[string]interface{} `json:"metadata"`
	Error          string                 `json:"error,omitempty"`
}

// BatchConversionResult represents the result of batch conversion
type BatchConversionResult struct {
	Results []ConversionResult `json:"results"`
	Summary struct {
		Total     int `json:"total"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
	} `json:"summary"`
}

// PyMuPDFConverter handles document to markdown conversion using PyMuPDF
type PyMuPDFConverter struct {
	scriptPath string
}

// NewPyMuPDFConverter creates a new PyMuPDF converter instance
func NewPyMuPDFConverter(scriptPath string) *PyMuPDFConverter {
	if scriptPath == "" {
		scriptPath = "./scripts"
	}

	return &PyMuPDFConverter{
		scriptPath: scriptPath,
	}
}

// ConvertToMarkdown converts a document to markdown format
func (p *PyMuPDFConverter) ConvertToMarkdown(inputPath, outputPath string, options ConversionOptions) (*ConversionResult, error) {
	startTime := time.Now()

	// Check if input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", inputPath)
	}

	// Python script path
	scriptFile := filepath.Join(p.scriptPath, "markdown_converter.py")

	// Check if Python script exists
	if _, err := os.Stat(scriptFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("PyMuPDF script not found: %s", scriptFile)
	}

	// If output path not specified, generate one
	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		outputPath = inputPath[:len(inputPath)-len(ext)] + ".md"
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Build command arguments
	args := []string{
		scriptFile,
		inputPath,
		"-o", outputPath,
		"--json",
	}

	// Add options
	if options.PreserveImages {
		args = append(args, "--preserve-images")
	}

	// Execute Python script
	cmd := exec.Command("python3", args...)
	cmd.Dir = filepath.Dir(scriptFile)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("PyMuPDF conversion failed: %v", err)
	}

	// Parse result
	var result ConversionResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse conversion result: %v", err)
	}

	// Check if conversion was successful
	if !result.Success {
		return &result, fmt.Errorf("conversion failed: %s", result.Error)
	}

	// Update timing
	result.Duration = time.Since(startTime).Seconds()

	return &result, nil
}

// BatchConvert converts multiple documents in a directory
func (p *PyMuPDFConverter) BatchConvert(inputDir, outputDir string, options ConversionOptions) (*BatchConversionResult, error) {
	// Check if input directory exists
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("input directory not found: %s", inputDir)
	}

	// Python script path
	scriptFile := filepath.Join(p.scriptPath, "markdown_converter.py")

	// Check if Python script exists
	if _, err := os.Stat(scriptFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("PyMuPDF script not found: %s", scriptFile)
	}

	// If output directory not specified, generate one
	if outputDir == "" {
		outputDir = inputDir + "_markdown"
	}

	// Build command arguments
	args := []string{
		scriptFile,
		inputDir,
		"-o", outputDir,
		"--batch",
		"--json",
	}

	// Add options
	if options.PreserveImages {
		args = append(args, "--preserve-images")
	}

	// Execute Python script
	cmd := exec.Command("python3", args...)
	cmd.Dir = filepath.Dir(scriptFile)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("PyMuPDF batch conversion failed: %v", err)
	}

	// Parse results
	var results []ConversionResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to parse batch conversion results: %v", err)
	}

	// Create batch result
	batchResult := &BatchConversionResult{
		Results: results,
	}

	// Calculate summary
	batchResult.Summary.Total = len(results)
	for _, result := range results {
		if result.Success {
			batchResult.Summary.Succeeded++
		} else {
			batchResult.Summary.Failed++
		}
	}

	return batchResult, nil
}

// ConvertPDFToMarkdown specifically converts PDF to markdown
func (p *PyMuPDFConverter) ConvertPDFToMarkdown(pdfPath, outputPath string, options ConversionOptions) (*ConversionResult, error) {
	// Validate PDF file
	if filepath.Ext(pdfPath) != ".pdf" {
		return nil, fmt.Errorf("input file is not a PDF: %s", pdfPath)
	}

	return p.ConvertToMarkdown(pdfPath, outputPath, options)
}

// ConvertOfficeToMarkdown converts Office documents to markdown
func (p *PyMuPDFConverter) ConvertOfficeToMarkdown(officePath, outputPath string, options ConversionOptions) (*ConversionResult, error) {
	// Validate office file extension
	ext := filepath.Ext(officePath)
	supportedExts := []string{".docx", ".doc", ".pptx", ".ppt", ".xlsx", ".xls", ".odt", ".odp", ".ods"}

	supported := false
	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			supported = true
			break
		}
	}

	if !supported {
		return nil, fmt.Errorf("unsupported office document format: %s", ext)
	}

	return p.ConvertToMarkdown(officePath, outputPath, options)
}

// ConversionOptions holds options for document conversion
type ConversionOptions struct {
	PreserveImages bool `json:"preserve_images"`
	TableStyle     bool `json:"table_style"`
	Columns        int  `json:"columns"`
}

// DefaultConversionOptions returns default conversion options
func DefaultConversionOptions() ConversionOptions {
	return ConversionOptions{
		PreserveImages: true,
		TableStyle:     true,
		Columns:        80,
	}
}
