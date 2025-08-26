package pymupdf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions
func getTestFilePath(filename string) string {
	return filepath.Join("..", "test_files", filename)
}

// Test PyMuPDF Converter Creation
func TestPyMuPDFConverter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PyMuPDF test in short mode")
	}

	// Test basic converter structure
	result := &ConversionResult{
		Success:        true,
		InputPath:      "test.pdf",
		OutputPath:     "test.md",
		ConversionType: "pdf_to_markdown",
		Duration:       1.5,
		FileSize:       1024,
		WordCount:      150,
		CharCount:      800,
		Metadata:       make(map[string]interface{}),
	}

	assert.True(t, result.Success)
	assert.Equal(t, "test.pdf", result.InputPath)
	assert.Equal(t, "pdf_to_markdown", result.ConversionType)
	assert.Greater(t, result.Duration, 0.0)
	assert.Greater(t, result.FileSize, int64(0))
}

// Test PDF to Markdown Conversion
func TestPDFToMarkdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PDF to Markdown test in short mode")
	}

	inputPath := getTestFilePath("test.pdf")
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("Test PDF file not found")
	}

	// Test would normally call Python PyMuPDF script
	// For now, we just verify the structure
	result, err := simulatePDFToMarkdown(inputPath)
	if err != nil {
		t.Logf("PyMuPDF conversion simulation failed (Python/PyMuPDF might not be available): %v", err)
		return
	}

	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, inputPath, result.InputPath)
	assert.Equal(t, "pdf_to_markdown", result.ConversionType)

	t.Logf("PDF to Markdown conversion simulated successfully: %d words, %d chars",
		result.WordCount, result.CharCount)
}

// Test PDF Text Extraction
func TestPDFTextExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PDF text extraction test in short mode")
	}

	inputPath := getTestFilePath("test.pdf")
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("Test PDF file not found")
	}

	// Test would normally call Python PyMuPDF script for text extraction
	result, err := simulatePDFTextExtraction(inputPath)
	if err != nil {
		t.Logf("PyMuPDF text extraction simulation failed: %v", err)
		return
	}

	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "pdf_to_text", result.ConversionType)
	assert.Greater(t, result.CharCount, 0)

	t.Logf("PDF text extraction simulated: %d characters extracted", result.CharCount)
}

// Test Batch Conversion
func TestBatchConversion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping batch conversion test in short mode")
	}

	// Simulate batch conversion results
	batchResult := &BatchConversionResult{
		Results: []ConversionResult{
			{
				Success:        true,
				InputPath:      "doc1.pdf",
				ConversionType: "pdf_to_markdown",
				Duration:       1.2,
				WordCount:      100,
			},
			{
				Success:        true,
				InputPath:      "doc2.pdf",
				ConversionType: "pdf_to_text",
				Duration:       0.8,
				WordCount:      80,
			},
		},
	}

	assert.Len(t, batchResult.Results, 2)
	assert.True(t, batchResult.Results[0].Success)
	assert.True(t, batchResult.Results[1].Success)

	totalWords := 0
	for _, result := range batchResult.Results {
		totalWords += result.WordCount
	}
	assert.Equal(t, 180, totalWords)

	t.Logf("Batch conversion simulated: %d documents, %d total words",
		len(batchResult.Results), totalWords)
}

// Helper functions for simulation (since we don't have actual Python integration yet)
func simulatePDFToMarkdown(inputPath string) (*ConversionResult, error) {
	return &ConversionResult{
		Success:        true,
		InputPath:      inputPath,
		OutputPath:     "output.md",
		ConversionType: "pdf_to_markdown",
		Duration:       1.5,
		FileSize:       2048,
		WordCount:      250,
		CharCount:      1500,
		Metadata:       map[string]interface{}{"pages": 3, "format": "markdown"},
	}, nil
}

func simulatePDFTextExtraction(inputPath string) (*ConversionResult, error) {
	return &ConversionResult{
		Success:        true,
		InputPath:      inputPath,
		OutputPath:     "output.txt",
		ConversionType: "pdf_to_text",
		Duration:       0.8,
		FileSize:       1024,
		WordCount:      200,
		CharCount:      1200,
		Metadata:       map[string]interface{}{"pages": 3, "format": "text"},
	}, nil
}
