package textextractor

import (
	"documents-worker/config"
	"documents-worker/utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type TextExtractor struct {
	config *config.ExternalConfig
}

type ExtractionResult struct {
	Text        string                 `json:"text"`
	SourceType  string                 `json:"source_type"`
	PageCount   int                    `json:"page_count"`
	WordCount   int                    `json:"word_count"`
	CharCount   int                    `json:"char_count"`
	Language    string                 `json:"language,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExtractedAt time.Time              `json:"extracted_at"`
	Duration    time.Duration          `json:"duration"`
}

type DocumentInfo struct {
	Title       string            `json:"title,omitempty"`
	Author      string            `json:"author,omitempty"`
	Subject     string            `json:"subject,omitempty"`
	Creator     string            `json:"creator,omitempty"`
	Producer    string            `json:"producer,omitempty"`
	CreatedDate string            `json:"created_date,omitempty"`
	ModDate     string            `json:"modified_date,omitempty"`
	Pages       int               `json:"pages"`
	Properties  map[string]string `json:"properties,omitempty"`
}

func NewTextExtractor(externalConfig *config.ExternalConfig) *TextExtractor {
	return &TextExtractor{
		config: externalConfig,
	}
}

// ExtractFromFile determines file type and extracts text accordingly
func (te *TextExtractor) ExtractFromFile(filePath string) (*ExtractionResult, error) {
	startTime := time.Now()

	// Detect MIME type
	mimeType, err := utils.DetectMimeTypeFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}

	var result *ExtractionResult

	switch {
	case utils.IsPdfDocument(mimeType):
		result, err = te.extractFromPDF(filePath)
	case utils.IsOfficeDocument(mimeType):
		result, err = te.extractFromOfficeDocument(filePath)
	case strings.Contains(mimeType, "text/"):
		result, err = te.extractFromTextFile(filePath)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", mimeType)
	}

	if err != nil {
		return nil, err
	}

	// Calculate statistics
	result.Duration = time.Since(startTime)
	result.ExtractedAt = time.Now()
	result.WordCount = te.countWords(result.Text)
	result.CharCount = len(result.Text)

	return result, nil
}

// extractFromPDF extracts text from PDF using MuPDF
func (te *TextExtractor) extractFromPDF(pdfPath string) (*ExtractionResult, error) {
	// First get PDF info
	info, err := te.getPDFInfo(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF info: %w", err)
	}

	// Extract text using mutool
	cmd := exec.Command(te.config.MutoolPath, "draw", "-F", "txt", pdfPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to extract text with mutool: %w", err)
	}

	text := string(output)

	// Clean up the text
	text = te.cleanExtractedText(text)

	result := &ExtractionResult{
		Text:       text,
		SourceType: "pdf",
		PageCount:  info.Pages,
		Metadata: map[string]interface{}{
			"source_file": filepath.Base(pdfPath),
			"pdf_info":    info,
			"extractor":   "mutool",
		},
	}

	return result, nil
}

// extractFromOfficeDocument converts to PDF first, then extracts text
func (te *TextExtractor) extractFromOfficeDocument(docPath string) (*ExtractionResult, error) {
	// Try direct text extraction first using LibreOffice headless mode
	text, err := te.extractTextWithLibreOffice(docPath)
	if err == nil && strings.TrimSpace(text) != "" {
		return &ExtractionResult{
			Text:       te.cleanExtractedText(text),
			SourceType: "office_direct",
			PageCount:  1, // We don't know page count from direct extraction
			Metadata: map[string]interface{}{
				"source_file": filepath.Base(docPath),
				"extractor":   "libreoffice_direct",
			},
		}, nil
	}

	// Fallback: Convert to PDF first, then extract
	pdfPath, err := te.convertToPDF(docPath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document to PDF: %w", err)
	}
	defer os.Remove(pdfPath)

	result, err := te.extractFromPDF(pdfPath)
	if err != nil {
		return nil, err
	}

	// Update metadata to reflect the conversion
	result.SourceType = "office_via_pdf"
	result.Metadata["conversion_method"] = "libreoffice_to_pdf"
	result.Metadata["original_file"] = filepath.Base(docPath)

	return result, nil
}

// extractTextWithLibreOffice directly extracts text using LibreOffice
func (te *TextExtractor) extractTextWithLibreOffice(docPath string) (string, error) {
	outputDir, err := os.MkdirTemp("", "libreoffice-text-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(outputDir)

	// Convert to plain text
	cmd := exec.Command(te.config.LibreOfficePath,
		"--headless",
		"--convert-to", "txt:Text",
		"--outdir", outputDir,
		docPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("libreoffice text extraction failed: %w, output: %s", err, string(output))
	}

	// Find the generated text file
	baseName := strings.TrimSuffix(filepath.Base(docPath), filepath.Ext(docPath))
	textFilePath := filepath.Join(outputDir, baseName+".txt")

	textBytes, err := os.ReadFile(textFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted text file: %w", err)
	}

	return string(textBytes), nil
}

// extractFromTextFile reads plain text files
func (te *TextExtractor) extractFromTextFile(textPath string) (*ExtractionResult, error) {
	content, err := os.ReadFile(textPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read text file: %w", err)
	}

	text := string(content)

	result := &ExtractionResult{
		Text:       text,
		SourceType: "plain_text",
		PageCount:  1,
		Metadata: map[string]interface{}{
			"source_file": filepath.Base(textPath),
			"extractor":   "direct_read",
		},
	}

	return result, nil
}

// getPDFInfo extracts metadata from PDF using mutool
func (te *TextExtractor) getPDFInfo(pdfPath string) (*DocumentInfo, error) {
	cmd := exec.Command(te.config.MutoolPath, "info", pdfPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF info: %w", err)
	}

	info := &DocumentInfo{
		Properties: make(map[string]string),
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse different info types
		if strings.Contains(line, "Pages:") {
			fmt.Sscanf(line, "Pages: %d", &info.Pages)
		} else if strings.HasPrefix(line, "Title:") {
			info.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title:"))
		} else if strings.HasPrefix(line, "Author:") {
			info.Author = strings.TrimSpace(strings.TrimPrefix(line, "Author:"))
		} else if strings.HasPrefix(line, "Subject:") {
			info.Subject = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		} else if strings.HasPrefix(line, "Creator:") {
			info.Creator = strings.TrimSpace(strings.TrimPrefix(line, "Creator:"))
		} else if strings.HasPrefix(line, "Producer:") {
			info.Producer = strings.TrimSpace(strings.TrimPrefix(line, "Producer:"))
		} else if strings.Contains(line, ":") {
			// Store other properties
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				info.Properties[key] = value
			}
		}
	}

	return info, nil
}

// convertToPDF converts office documents to PDF using LibreOffice
func (te *TextExtractor) convertToPDF(docPath string) (string, error) {
	outputDir, err := os.MkdirTemp("", "pdf-convert-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	cmd := exec.Command(te.config.LibreOfficePath,
		"--headless",
		"--convert-to", "pdf",
		"--outdir", outputDir,
		docPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(outputDir)
		return "", fmt.Errorf("libreoffice PDF conversion failed: %w, output: %s", err, string(output))
	}

	// Construct expected PDF path
	baseName := strings.TrimSuffix(filepath.Base(docPath), filepath.Ext(docPath))
	pdfPath := filepath.Join(outputDir, baseName+".pdf")

	// Verify PDF was created
	if _, err := os.Stat(pdfPath); err != nil {
		os.RemoveAll(outputDir)
		return "", fmt.Errorf("PDF file was not created: %w", err)
	}

	return pdfPath, nil
}

// cleanExtractedText cleans up extracted text
func (te *TextExtractor) cleanExtractedText(text string) string {
	// Remove excessive whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Remove control characters except newlines and tabs
	text = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`).ReplaceAllString(text, "")

	// Clean up multiple consecutive newlines
	text = regexp.MustCompile(`\n\s*\n\s*\n`).ReplaceAllString(text, "\n\n")

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}

// countWords counts words in text
func (te *TextExtractor) countWords(text string) int {
	if text == "" {
		return 0
	}

	// Split by whitespace and filter empty strings
	words := strings.Fields(text)
	return len(words)
}

// ExtractByPages extracts text from specific PDF pages
func (te *TextExtractor) ExtractByPages(pdfPath string, startPage, endPage int) (*ExtractionResult, error) {
	if endPage < startPage {
		return nil, fmt.Errorf("end page cannot be less than start page")
	}

	startTime := time.Now()

	// Get PDF info first
	info, err := te.getPDFInfo(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF info: %w", err)
	}

	if startPage > info.Pages || endPage > info.Pages {
		return nil, fmt.Errorf("page range exceeds document pages (%d)", info.Pages)
	}

	// Extract text from specific pages
	pageRange := fmt.Sprintf("%d-%d", startPage, endPage)
	cmd := exec.Command(te.config.MutoolPath, "draw", "-F", "txt", pdfPath, pageRange)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from pages %s: %w", pageRange, err)
	}

	text := te.cleanExtractedText(string(output))

	result := &ExtractionResult{
		Text:        text,
		SourceType:  "pdf_pages",
		PageCount:   endPage - startPage + 1,
		WordCount:   te.countWords(text),
		CharCount:   len(text),
		ExtractedAt: time.Now(),
		Duration:    time.Since(startTime),
		Metadata: map[string]interface{}{
			"source_file": filepath.Base(pdfPath),
			"page_range":  pageRange,
			"start_page":  startPage,
			"end_page":    endPage,
			"total_pages": info.Pages,
			"extractor":   "mutool_pages",
			"pdf_info":    info,
		},
	}

	return result, nil
}

// BatchExtractPDFPages extracts text from each page separately
func (te *TextExtractor) BatchExtractPDFPages(pdfPath string) ([]*ExtractionResult, error) {
	// Get PDF info
	info, err := te.getPDFInfo(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF info: %w", err)
	}

	var results []*ExtractionResult

	for page := 1; page <= info.Pages; page++ {
		result, err := te.ExtractByPages(pdfPath, page, page)
		if err != nil {
			// Log error but continue with other pages
			fmt.Printf("Failed to extract page %d: %v\n", page, err)
			continue
		}

		// Update metadata for individual page
		result.Metadata["page_number"] = page
		result.SourceType = "pdf_page"

		results = append(results, result)
	}

	return results, nil
}
