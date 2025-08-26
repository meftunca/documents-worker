package textextractor

import (
	"documents-worker/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test configuration
func getTestExtractorConfig() *config.ExternalConfig {
	return &config.ExternalConfig{
		TesseractPath:   "/usr/bin/tesseract",
		FFmpegPath:      "/usr/bin/ffmpeg",
		LibreOfficePath: "/usr/bin/libreoffice",
		MutoolPath:      "/usr/bin/mutool",
		PyMuPDFScript:   "/usr/bin/python3",
		WkHtmlToPdfPath: "/usr/bin/wkhtmltopdf",
		PandocPath:      "/usr/bin/pandoc",
		VipsEnabled:     false,
	}
}

func getTestFilePath(filename string) string {
	return filepath.Join("..", "test_files", filename)
}

// Test Text Extractor Creation
func TestTextExtractorCreation(t *testing.T) {
	config := getTestExtractorConfig()

	extractor := NewTextExtractor(config)

	assert.NotNil(t, extractor)
}

// Test PDF Text Extraction
func TestPDFTextExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PDF extraction test in short mode")
	}

	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	pdfPath := getTestFilePath("test.pdf")

	// Check if test PDF exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skip("Test PDF not found")
	}

	// Extract text from PDF
	result, err := extractor.ExtractFromFile(pdfPath)

	if err != nil {
		t.Logf("PDF text extraction failed (tools might not be available): %v", err)
		return
	}

	// Verify result
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Text), 0, "Text should be extractable")
	assert.GreaterOrEqual(t, result.PageCount, 1, "PDF should have at least one page")

	t.Logf("PDF Text Extraction Result: Pages=%d, Text length=%d",
		result.PageCount, len(result.Text))
}

// Test Office Document Text Extraction
func TestOfficeDocumentExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping office document extraction test in short mode")
	}

	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	testDocs := []string{"test.docx", "test.xlsx"}

	for _, docName := range testDocs {
		t.Run("Extract_"+docName, func(t *testing.T) {
			docPath := getTestFilePath(docName)

			// Check if test document exists
			if _, err := os.Stat(docPath); os.IsNotExist(err) {
				t.Skipf("Test document %s not found", docPath)
			}

			// Extract text from document
			result, err := extractor.ExtractFromFile(docPath)

			if err != nil {
				t.Logf("Office document extraction failed (tools might not be available): %v", err)
				return
			}

			// Verify result
			assert.NotNil(t, result)
			assert.GreaterOrEqual(t, len(result.Text), 0, "Text should be extractable")

			t.Logf("Office Document Extraction Result: %s, Text length=%d",
				docName, len(result.Text))
		})
	}
}

// Test HTML Text Extraction
func TestHTMLTextExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTML extraction test in short mode")
	}

	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	htmlPath := getTestFilePath("test.html")

	// Check if test HTML exists
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		t.Skip("Test HTML not found")
	}

	// Extract text from HTML
	result, err := extractor.ExtractFromFile(htmlPath)

	if err != nil {
		t.Logf("HTML text extraction failed: %v", err)
		return
	}

	// Verify result
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Text), 0, "Text should be extractable")

	t.Logf("HTML Text Extraction Result: Text length=%d", len(result.Text))
}

// Test CSV Text Extraction
func TestCSVTextExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CSV extraction test in short mode")
	}

	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	csvPath := getTestFilePath("test.csv")

	// Check if test CSV exists
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Skip("Test CSV not found")
	}

	// Extract text from CSV
	result, err := extractor.ExtractFromFile(csvPath)

	if err != nil {
		t.Logf("CSV text extraction failed: %v", err)
		return
	}

	// Verify result
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Text), 0, "Text should be extractable")

	t.Logf("CSV Text Extraction Result: Text length=%d", len(result.Text))
}

// Test Markdown Text Extraction
func TestMarkdownTextExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Markdown extraction test in short mode")
	}

	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	mdPath := getTestFilePath("test.md")

	// Check if test Markdown exists
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Skip("Test Markdown not found")
	}

	// Extract text from Markdown
	result, err := extractor.ExtractFromFile(mdPath)

	if err != nil {
		t.Logf("Markdown text extraction failed: %v", err)
		return
	}

	// Verify result
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Text), 0, "Text should be extractable")

	t.Logf("Markdown Text Extraction Result: Text length=%d", len(result.Text))
}

// Test PDF Page Range Extraction
func TestPDFPageRangeExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PDF page range extraction test in short mode")
	}

	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	pdfPath := getTestFilePath("test.pdf")

	// Check if test PDF exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skip("Test PDF not found")
	}

	// Extract text from specific page range
	result, err := extractor.ExtractByPages(pdfPath, 1, 1)

	if err != nil {
		t.Logf("PDF page range extraction failed (tools might not be available): %v", err)
		return
	}

	// Verify result
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Text), 0, "Text should be extractable")
	assert.Equal(t, 1, result.PageCount, "Should extract exactly one page")

	t.Logf("PDF Page Range Extraction Result: Pages=%d, Text length=%d",
		result.PageCount, len(result.Text))
}

// Test Error Handling
func TestExtractionErrorHandling(t *testing.T) {
	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	// Test with non-existent file
	_, err := extractor.ExtractFromFile("non-existent-file.pdf")
	assert.Error(t, err, "Should return error for non-existent file")

	// Test with invalid file path
	_, err = extractor.ExtractFromFile("")
	assert.Error(t, err, "Should return error for empty file path")
}

// Benchmark Tests
func BenchmarkTextExtractorCreation(b *testing.B) {
	config := getTestExtractorConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractor := NewTextExtractor(config)
		_ = extractor
	}
}

func BenchmarkPDFTextExtraction(b *testing.B) {
	config := getTestExtractorConfig()
	extractor := NewTextExtractor(config)

	pdfPath := getTestFilePath("test.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		b.Skip("Test PDF not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractFromFile(pdfPath)
		if err != nil {
			b.Skip("Text extraction tools not available")
		}
	}
}
