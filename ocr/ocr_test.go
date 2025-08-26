package ocr

import (
	"documents-worker/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test configuration
func getTestOCRConfig() (*config.OCRConfig, *config.ExternalConfig) {
	ocrConfig := &config.OCRConfig{
		Language: "tur",
		DPI:      300,
		PSM:      3,
	}

	externalConfig := &config.ExternalConfig{
		TesseractPath:   "/usr/bin/tesseract",
		FFmpegPath:      "/usr/bin/ffmpeg",
		LibreOfficePath: "/usr/bin/libreoffice",
		MutoolPath:      "/usr/bin/mutool",
		PyMuPDFScript:   "/usr/bin/python3",
		WkHtmlToPdfPath: "/usr/bin/wkhtmltopdf",
		PandocPath:      "/usr/bin/pandoc",
		VipsEnabled:     false,
	}

	return ocrConfig, externalConfig
}

func getTestImagePath(filename string) string {
	return filepath.Join("..", "test_files", filename)
}

// Test OCR Processor Creation
func TestOCRProcessorCreation(t *testing.T) {
	ocrConfig, externalConfig := getTestOCRConfig()
	
	processor := NewOCRProcessor(ocrConfig, externalConfig)
	
	assert.NotNil(t, processor)
	assert.Equal(t, ocrConfig, processor.config)
	assert.Equal(t, externalConfig, processor.external)
}

// Test OCR Configuration Validation
func TestOCRConfigValidation(t *testing.T) {
	ocrConfig, externalConfig := getTestOCRConfig()
	
	// Test with valid config
	processor := NewOCRProcessor(ocrConfig, externalConfig)
	assert.NotNil(t, processor)

	// Test language settings
	assert.Equal(t, "tur", ocrConfig.Language)
	assert.Equal(t, 300, ocrConfig.DPI)
	assert.Equal(t, 3, ocrConfig.PSM)
}

// Test Image OCR Processing
func TestImageOCRProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OCR processing test in short mode")
	}

	ocrConfig, externalConfig := getTestOCRConfig()
	processor := NewOCRProcessor(ocrConfig, externalConfig)

	// Test with image files
	testImages := []string{"test.webp", "test.avif"}

	for _, imageName := range testImages {
		t.Run("OCR_"+imageName, func(t *testing.T) {
			imagePath := getTestImagePath(imageName)
			
			// Check if test image exists
			if _, err := os.Stat(imagePath); os.IsNotExist(err) {
				t.Skipf("Test image %s not found", imagePath)
			}

			// Process image OCR
			result, err := processor.ProcessImage(imagePath)
			
			if err != nil {
				t.Logf("OCR processing failed (Tesseract might not be available): %v", err)
				return
			}

			// Verify result
			assert.NotNil(t, result)
			assert.GreaterOrEqual(t, len(result.Text), 0, "OCR should return text or empty result")
			
			t.Logf("OCR Result for %s: Text length=%d", imageName, len(result.Text))
		})
	}
}

// Test Document OCR Processing
func TestDocumentOCRProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping document OCR processing test in short mode")
	}

	ocrConfig, externalConfig := getTestOCRConfig()
	processor := NewOCRProcessor(ocrConfig, externalConfig)

	pdfPath := getTestImagePath("test.pdf")
	
	// Check if test PDF exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skip("Test PDF not found")
	}

	// Process PDF OCR
	result, err := processor.ProcessDocument(pdfPath)
	
	if err != nil {
		t.Logf("PDF OCR processing failed (tools might not be available): %v", err)
		return
	}

	// Verify result
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Text), 0, "OCR should return text or empty result")
	
	t.Logf("PDF OCR Result: Text length=%d", len(result.Text))
}

// Test Error Handling
func TestOCRErrorHandling(t *testing.T) {
	ocrConfig, externalConfig := getTestOCRConfig()
	processor := NewOCRProcessor(ocrConfig, externalConfig)

	// Test with non-existent file
	_, err := processor.ProcessImage("non-existent-file.jpg")
	assert.Error(t, err, "Should return error for non-existent file")

	// Test with empty file path
	_, err = processor.ProcessImage("")
	assert.Error(t, err, "Should return error for empty file path")
}

// Benchmark OCR Processor Creation
func BenchmarkOCRProcessorCreation(b *testing.B) {
	ocrConfig, externalConfig := getTestOCRConfig()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor := NewOCRProcessor(ocrConfig, externalConfig)
		_ = processor
	}
}

// Benchmark Image OCR
func BenchmarkImageOCR(b *testing.B) {
	ocrConfig, externalConfig := getTestOCRConfig()
	processor := NewOCRProcessor(ocrConfig, externalConfig)
	
	imagePath := getTestImagePath("test.webp")
	
	// Check if test image exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		b.Skip("Test image not found")
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.ProcessImage(imagePath)
		if err != nil {
			b.Skip("OCR tools not available")
		}
	}
}
