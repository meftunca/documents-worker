package processors

import (
	"context"
	"documents-worker/config"
	"documents-worker/internal/core/ports"
	"documents-worker/ocr"
	"documents-worker/pdfgen"
	"documents-worker/textextractor"
	"fmt"
	"io"
	"os"
	"strings"
)

// PlaywrightPDFProcessor implements the PDFProcessor port using Playwright
type PlaywrightPDFProcessor struct {
	generator *pdfgen.PDFGenerator
}

// NewPlaywrightPDFProcessor creates a new Playwright PDF processor
func NewPlaywrightPDFProcessor(externalConfig *config.ExternalConfig) ports.PDFProcessor {
	generator := pdfgen.NewPDFGenerator(externalConfig)

	return &PlaywrightPDFProcessor{
		generator: generator,
	}
}

// GenerateFromHTML generates a PDF from HTML content
func (p *PlaywrightPDFProcessor) GenerateFromHTML(ctx context.Context, html io.Reader, params map[string]interface{}) (io.Reader, error) {
	// Create temporary HTML file
	htmlFile, err := os.CreateTemp("", "input-*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer os.Remove(htmlFile.Name())
	defer htmlFile.Close()

	// Copy HTML content to temp file
	_, err = io.Copy(htmlFile, html)
	if err != nil {
		return nil, fmt.Errorf("failed to copy HTML content: %w", err)
	}

	// Prepare generation options
	options := &pdfgen.GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		Margins: map[string]string{
			"top":    "1cm",
			"bottom": "1cm",
			"left":   "1cm",
			"right":  "1cm",
		},
	}

	// Apply parameters
	if pageSize, ok := params["page_size"].(string); ok {
		options.PageSize = pageSize
	}
	if orientation, ok := params["orientation"].(string); ok {
		options.Orientation = orientation
	}

	// Generate PDF using the file path directly
	result, err := p.generator.GenerateFromHTMLFileWithPlaywright(htmlFile.Name(), options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF with Playwright: %w", err)
	}

	// Open the generated PDF file
	pdfFile, err := os.Open(result.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open generated PDF: %w", err)
	}

	return pdfFile, nil
}

// GenerateFromURL generates a PDF from a URL
func (p *PlaywrightPDFProcessor) GenerateFromURL(ctx context.Context, url string, params map[string]interface{}) (io.Reader, error) {
	// Prepare generation options
	options := &pdfgen.GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		Margins: map[string]string{
			"top":    "1cm",
			"bottom": "1cm",
			"left":   "1cm",
			"right":  "1cm",
		},
	}

	// Apply parameters
	if pageSize, ok := params["page_size"].(string); ok {
		options.PageSize = pageSize
	}
	if orientation, ok := params["orientation"].(string); ok {
		options.Orientation = orientation
	}

	// Generate PDF from URL
	result, err := p.generator.GenerateFromURLWithPlaywright(url, options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF from URL with Playwright: %w", err)
	}

	// Open the generated PDF file
	pdfFile, err := os.Open(result.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open generated PDF: %w", err)
	}

	return pdfFile, nil
}

// ExtractText extracts text from a PDF
func (p *PlaywrightPDFProcessor) ExtractText(ctx context.Context, input io.Reader) (string, error) {
	// Create temporary PDF file
	pdfFile, err := os.CreateTemp("", "input-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	defer os.Remove(pdfFile.Name())
	defer pdfFile.Close()

	// Copy PDF content to temp file
	_, err = io.Copy(pdfFile, input)
	if err != nil {
		return "", fmt.Errorf("failed to copy PDF content: %w", err)
	}

	// Use a default config for text extraction
	externalConfig := &config.ExternalConfig{}
	extractor := textextractor.NewTextExtractor(externalConfig)
	result, err := extractor.ExtractFromFile(pdfFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	return result.Text, nil
}

// GetPageCount returns the number of pages in a PDF
func (p *PlaywrightPDFProcessor) GetPageCount(ctx context.Context, input io.Reader) (int, error) {
	// Create temporary PDF file
	pdfFile, err := os.CreateTemp("", "input-*.pdf")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	defer os.Remove(pdfFile.Name())
	defer pdfFile.Close()

	// Copy PDF content to temp file
	_, err = io.Copy(pdfFile, input)
	if err != nil {
		return 0, fmt.Errorf("failed to copy PDF content: %w", err)
	}

	// Use a default config for text extraction
	externalConfig := &config.ExternalConfig{}
	extractor := textextractor.NewTextExtractor(externalConfig)
	result, err := extractor.ExtractFromFile(pdfFile.Name())
	if err != nil {
		return 0, fmt.Errorf("failed to get PDF info: %w", err)
	}

	return result.PageCount, nil
}

// TesseractOCRProcessor implements the OCRProcessor port using Tesseract
type TesseractOCRProcessor struct {
	processor *ocr.OCRProcessor
}

// NewTesseractOCRProcessor creates a new Tesseract OCR processor
func NewTesseractOCRProcessor(ocrConfig *config.OCRConfig, externalConfig *config.ExternalConfig) ports.OCRProcessor {
	processor := ocr.NewOCRProcessor(ocrConfig, externalConfig)

	return &TesseractOCRProcessor{
		processor: processor,
	}
}

// ProcessImage performs OCR on an image
func (p *TesseractOCRProcessor) ProcessImage(ctx context.Context, input io.Reader, language string) (string, error) {
	// Create temporary image file
	imageFile, err := os.CreateTemp("", "input-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create temp image file: %w", err)
	}
	defer os.Remove(imageFile.Name())
	defer imageFile.Close()

	// Copy image content to temp file
	_, err = io.Copy(imageFile, input)
	if err != nil {
		return "", fmt.Errorf("failed to copy image content: %w", err)
	}

	// Perform OCR (note: current API doesn't support language parameter)
	result, err := p.processor.ProcessImage(imageFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to perform OCR on image: %w", err)
	}

	return result.Text, nil
}

// ProcessPDF performs OCR on a PDF
func (p *TesseractOCRProcessor) ProcessPDF(ctx context.Context, input io.Reader, language string) (string, error) {
	// Create temporary PDF file
	pdfFile, err := os.CreateTemp("", "input-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	defer os.Remove(pdfFile.Name())
	defer pdfFile.Close()

	// Copy PDF content to temp file
	_, err = io.Copy(pdfFile, input)
	if err != nil {
		return "", fmt.Errorf("failed to copy PDF content: %w", err)
	}

	// Perform OCR on PDF first page (note: current API expects page number)
	result, err := p.processor.ProcessPDF(pdfFile.Name(), 1)
	if err != nil {
		return "", fmt.Errorf("failed to perform OCR on PDF: %w", err)
	}

	return result.Text, nil
}

// GetSupportedLanguages returns the list of supported OCR languages
func (p *TesseractOCRProcessor) GetSupportedLanguages() []string {
	return []string{"eng", "tur", "fra", "deu", "spa", "ita", "por", "rus", "ara", "chi_sim", "chi_tra", "jpn", "kor"}
}

// MultiTextExtractor implements the TextExtractor port
type MultiTextExtractor struct {
	extractor *textextractor.TextExtractor
}

// NewMultiTextExtractor creates a new text extractor
func NewMultiTextExtractor(externalConfig *config.ExternalConfig) ports.TextExtractor {
	extractor := textextractor.NewTextExtractor(externalConfig)

	return &MultiTextExtractor{
		extractor: extractor,
	}
}

// ExtractFromOffice extracts text from Office documents
func (p *MultiTextExtractor) ExtractFromOffice(ctx context.Context, input io.Reader, docType string) (string, error) {
	// Create temporary file with appropriate extension
	var ext string
	switch strings.ToLower(docType) {
	case "docx", "doc":
		ext = ".docx"
	case "xlsx", "xls":
		ext = ".xlsx"
	case "pptx", "ppt":
		ext = ".pptx"
	default:
		ext = ".docx"
	}

	officeFile, err := os.CreateTemp("", "input-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp office file: %w", err)
	}
	defer os.Remove(officeFile.Name())
	defer officeFile.Close()

	// Copy content to temp file
	_, err = io.Copy(officeFile, input)
	if err != nil {
		return "", fmt.Errorf("failed to copy office content: %w", err)
	}

	// Extract text using the general ExtractFromFile method
	result, err := p.extractor.ExtractFromFile(officeFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to extract text from office document: %w", err)
	}

	return result.Text, nil
}

// ExtractFromPDF extracts text from PDF documents
func (p *MultiTextExtractor) ExtractFromPDF(ctx context.Context, input io.Reader) (string, error) {
	// Create temporary PDF file
	pdfFile, err := os.CreateTemp("", "input-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	defer os.Remove(pdfFile.Name())
	defer pdfFile.Close()

	// Copy content to temp file
	_, err = io.Copy(pdfFile, input)
	if err != nil {
		return "", fmt.Errorf("failed to copy PDF content: %w", err)
	}

	// Extract text using the general ExtractFromFile method
	result, err := p.extractor.ExtractFromFile(pdfFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	return result.Text, nil
}

// ExtractFromText extracts text from plain text files
func (p *MultiTextExtractor) ExtractFromText(ctx context.Context, input io.Reader) (string, error) {
	// Create temporary text file
	textFile, err := os.CreateTemp("", "input-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp text file: %w", err)
	}
	defer os.Remove(textFile.Name())
	defer textFile.Close()

	// Copy content to temp file
	_, err = io.Copy(textFile, input)
	if err != nil {
		return "", fmt.Errorf("failed to copy text content: %w", err)
	}

	// Read text content
	content, err := os.ReadFile(textFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read text file: %w", err)
	}

	return string(content), nil
}
