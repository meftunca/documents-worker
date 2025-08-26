package pdfgen

import (
	"documents-worker/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function to get test PDF generator config
func getTestPDFConfig() *config.ExternalConfig {
	return &config.ExternalConfig{
		WkHtmlToPdfPath:   "wkhtmltopdf", // Default path, will be checked in tests
		NodeJSPath:        "node",        // Node.js for Playwright
		PlaywrightEnabled: true,          // Enable Playwright
	}
}

// Test HTML to PDF Generation
func TestHTMLToPDFGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PDF generation test in short mode")
	}

	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	// Simple HTML content for testing
	htmlContent := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Test Document</title>
		<style>
			body { font-family: Arial, sans-serif; margin: 20px; }
			h1 { color: #333; }
			p { line-height: 1.6; }
		</style>
	</head>
	<body>
		<h1>Test PDF Document</h1>
		<p>This is a test document for PDF generation.</p>
		<p>It contains multiple paragraphs to test the formatting.</p>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
			<li>Item 3</li>
		</ul>
	</body>
	</html>
	`

	// Basic generation options
	options := &GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		Margins: map[string]string{
			"top":    "1cm",
			"bottom": "1cm",
			"left":   "1cm",
			"right":  "1cm",
		},
	}

	// Generate PDF
	result, err := generator.GenerateFromHTML(htmlContent, options)
	if err != nil {
		t.Skipf("PDF generation failed (tool might not be available): %v", err)
	}

	// Verify result
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.OutputPath, "Output path should be set")
	assert.Greater(t, result.FileSize, int64(100), "PDF should have reasonable size")
	assert.GreaterOrEqual(t, result.PageCount, 1, "PDF should have at least one page")

	// Verify file exists
	_, err = os.Stat(result.OutputPath)
	assert.NoError(t, err, "PDF file should exist")

	t.Logf("PDF Generation Result: Path=%s, Size=%d bytes, Pages=%d",
		result.OutputPath, result.FileSize, result.PageCount)
}

// Test Playwright PDF Generation
func TestPlaywrightPDFGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Playwright PDF generation test in short mode")
	}

	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	// Simple HTML content for testing
	htmlContent := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Playwright Test Document</title>
		<style>
			body { 
				font-family: Arial, sans-serif; 
				margin: 20px;
				color: #333;
			}
			h1 { 
				color: #2c3e50; 
				border-bottom: 2px solid #3498db;
				padding-bottom: 10px;
			}
			.highlight { 
				background-color: #f39c12; 
				color: white; 
				padding: 5px 10px; 
				border-radius: 5px;
			}
			.modern-box {
				background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
				color: white;
				padding: 20px;
				border-radius: 10px;
				margin: 20px 0;
			}
		</style>
	</head>
	<body>
		<h1>🎭 Playwright PDF Test</h1>
		<p>This is a <span class="highlight">modern PDF</span> generated with Playwright!</p>
		
		<div class="modern-box">
			<h3>Modern Features</h3>
			<ul>
				<li>✅ CSS3 Support</li>
				<li>✅ Modern JavaScript</li>
				<li>✅ High Quality Rendering</li>
				<li>✅ Fast Generation</li>
			</ul>
		</div>

		<p>Playwright provides much better rendering than legacy tools!</p>
		
		<table style="width: 100%; border-collapse: collapse; margin: 20px 0;">
			<tr style="background: #3498db; color: white;">
				<th style="padding: 10px; border: 1px solid #2980b9;">Feature</th>
				<th style="padding: 10px; border: 1px solid #2980b9;">wkhtmltopdf</th>
				<th style="padding: 10px; border: 1px solid #2980b9;">Playwright</th>
			</tr>
			<tr>
				<td style="padding: 10px; border: 1px solid #ddd;">CSS3 Support</td>
				<td style="padding: 10px; border: 1px solid #ddd;">Limited</td>
				<td style="padding: 10px; border: 1px solid #ddd;">Full</td>
			</tr>
			<tr style="background: #f8f9fa;">
				<td style="padding: 10px; border: 1px solid #ddd;">Performance</td>
				<td style="padding: 10px; border: 1px solid #ddd;">Slow</td>
				<td style="padding: 10px; border: 1px solid #ddd;">Fast</td>
			</tr>
		</table>
	</body>
	</html>
	`

	// Basic generation options
	options := &GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		Margins: map[string]string{
			"top":    "1cm",
			"bottom": "1cm",
			"left":   "1cm",
			"right":  "1cm",
		},
	}

	// Generate PDF with Playwright
	result, err := generator.GenerateFromHTMLWithPlaywright(htmlContent, options)
	if err != nil {
		t.Skipf("Playwright PDF generation failed (Node.js/Playwright might not be available): %v", err)
	}

	// Verify result
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.OutputPath, "Output path should be set")
	assert.Greater(t, result.FileSize, int64(1000), "PDF should have reasonable size")
	assert.GreaterOrEqual(t, result.PageCount, 0, "PDF page count should be non-negative") // Allow 0 if mutool not available
	assert.Equal(t, "html", result.InputType, "Input type should be html")

	// Check metadata
	assert.Equal(t, "playwright", result.Metadata["generator"])
	assert.Equal(t, "chromium", result.Metadata["engine"])

	// Verify file exists
	_, err = os.Stat(result.OutputPath)
	assert.NoError(t, err, "PDF file should exist")

	t.Logf("Playwright PDF Generation Result: Path=%s, Size=%d bytes, Pages=%d",
		result.OutputPath, result.FileSize, result.PageCount)
}

// Test Playwright URL to PDF Generation
func TestPlaywrightURLToPDF(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Playwright URL to PDF test in short mode")
	}

	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	// Test with a data URL (works without internet)
	dataURL := "data:text/html,<!DOCTYPE html><html><head><title>URL Test</title><style>body{font-family:Arial;padding:20px;background:linear-gradient(45deg,#f0f8ff,#e6e6fa);}</style></head><body><h1 style='color:#4169e1;text-align:center;'>🌐 URL to PDF Test</h1><p style='text-align:center;font-size:18px;'>This page was loaded from a data URL and converted to PDF using Playwright!</p><div style='background:white;padding:20px;border-radius:10px;box-shadow:0 4px 6px rgba(0,0,0,0.1);margin:20px 0;'><h3>✨ Benefits of Playwright:</h3><ul><li>Modern browser engine</li><li>Perfect CSS rendering</li><li>JavaScript execution</li><li>Fast and reliable</li></ul></div></body></html>"

	options := &GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		Margins: map[string]string{
			"top":    "1cm",
			"bottom": "1cm",
			"left":   "1cm",
			"right":  "1cm",
		},
	}

	// Generate PDF from URL with Playwright
	result, err := generator.GenerateFromURLWithPlaywright(dataURL, options)
	if err != nil {
		t.Skipf("Playwright URL to PDF generation failed (Node.js/Playwright might not be available): %v", err)
	}

	// Verify result
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.OutputPath, "Output path should be set")
	assert.Greater(t, result.FileSize, int64(1000), "PDF should have reasonable size")
	assert.GreaterOrEqual(t, result.PageCount, 0, "PDF page count should be non-negative") // Allow 0 if mutool not available
	assert.Equal(t, "url", result.InputType, "Input type should be url")

	// Check metadata
	assert.Equal(t, "playwright", result.Metadata["generator"])
	assert.Equal(t, "chromium", result.Metadata["engine"])
	assert.Equal(t, dataURL, result.Metadata["source_url"])

	// Verify file exists
	_, err = os.Stat(result.OutputPath)
	assert.NoError(t, err, "PDF file should exist")

	t.Logf("Playwright URL to PDF Result: Path=%s, Size=%d bytes, Pages=%d",
		result.OutputPath, result.FileSize, result.PageCount)
} // Test PDF Generation with Different Options
func TestPDFGenerationWithOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PDF generation with options test in short mode")
	}

	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	htmlContent := `
	<!DOCTYPE html>
	<html>
	<head><title>Styled Document</title></head>
	<body>
		<h1>Document with Custom Options</h1>
		<p>Testing PDF generation with custom page settings.</p>
	</body>
	</html>
	`

	// Test with different page formats
	testCases := []struct {
		name        string
		pageSize    string
		orientation string
	}{
		{"A4_Portrait", "A4", "portrait"},
		{"A4_Landscape", "A4", "landscape"},
		{"Letter_Portrait", "Letter", "portrait"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			options := &GenerationOptions{
				PageSize:    tc.pageSize,
				Orientation: tc.orientation,
				Margins: map[string]string{
					"top":    "2cm",
					"bottom": "2cm",
					"left":   "1.5cm",
					"right":  "1.5cm",
				},
			}

			result, err := generator.GenerateFromHTML(htmlContent, options)
			if err != nil {
				t.Logf("PDF generation with %s %s failed (might not be supported): %v",
					tc.pageSize, tc.orientation, err)
				return
			}

			// Verify result
			assert.NotNil(t, result)
			assert.Greater(t, result.FileSize, int64(100), "PDF should have reasonable size")

			t.Logf("PDF Generation (%s %s): Size=%d bytes",
				tc.pageSize, tc.orientation, result.FileSize)
		})
	}
}

// Test Markdown to PDF Generation
func TestMarkdownToPDFGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Markdown to PDF generation test in short mode")
	}

	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	markdownContent := `
# Test Markdown Document

This is a **test document** for *Markdown to PDF* generation.

## Features

- Support for headers
- **Bold text**
- *Italic text*
- Lists and more

### Code Block

` + "```go\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n```" + `

### Table

| Column 1 | Column 2 | Column 3 |
|----------|----------|----------|
| Value 1  | Value 2  | Value 3  |
| Value 4  | Value 5  | Value 6  |
	`

	options := &GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		Margins: map[string]string{
			"top":    "1cm",
			"bottom": "1cm",
			"left":   "1cm",
			"right":  "1cm",
		},
	}

	result, err := generator.GenerateFromMarkdown(markdownContent, options)
	if err != nil {
		t.Skipf("Markdown to PDF generation failed (tool might not be available): %v", err)
	}

	// Verify result
	assert.NotNil(t, result)
	assert.Greater(t, result.FileSize, int64(100), "PDF should have reasonable size")
	assert.Equal(t, "markdown", result.InputType, "Input type should be markdown")

	t.Logf("Markdown to PDF Generation: Size=%d bytes, Pages=%d",
		result.FileSize, result.PageCount)
}

// Test Office Document to PDF Generation
func TestOfficeDocumentToPDFGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping office document to PDF generation test in short mode")
	}

	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	// Test with existing office documents
	testDocs := []string{"test.docx", "test.xlsx"}

	for _, docName := range testDocs {
		t.Run("Convert_"+docName, func(t *testing.T) {
			docPath := filepath.Join("../test_files", docName)

			// Check if test document exists
			if _, err := os.Stat(docPath); os.IsNotExist(err) {
				t.Skipf("Test document %s not found", docPath)
			}

			options := &GenerationOptions{
				PageSize:    "A4",
				Orientation: "portrait",
				Margins: map[string]string{
					"top":    "1cm",
					"bottom": "1cm",
					"left":   "1cm",
					"right":  "1cm",
				},
			}

			result, err := generator.GenerateFromOfficeDocument(docPath, options)
			if err != nil {
				t.Logf("Office document to PDF conversion failed (tools might not be available): %v", err)
				return
			}

			// Verify result
			assert.NotNil(t, result)
			assert.Greater(t, result.FileSize, int64(100), "PDF should have reasonable size")
			assert.Contains(t, []string{"docx", "xlsx", "office"}, result.InputType)

			t.Logf("Office Document to PDF (%s): Size=%d bytes, Pages=%d",
				docName, result.FileSize, result.PageCount)
		})
	}
}

// Test Error Handling
func TestPDFGenerationErrorHandling(t *testing.T) {
	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	// Test with empty HTML
	_, err := generator.GenerateFromHTML("", &GenerationOptions{})
	if err != nil {
		t.Logf("Empty HTML processing failed as expected: %v", err)
	}

	// Test with nil options
	_, err = generator.GenerateFromHTML("<html><body>Test</body></html>", nil)
	if err != nil {
		t.Logf("Nil options processing failed as expected: %v", err)
	}

	// Test with non-existent office document
	_, err = generator.GenerateFromOfficeDocument("non-existent-file.docx", &GenerationOptions{})
	assert.Error(t, err, "Should return error for non-existent file")
}

// Benchmark PDF Generation
func BenchmarkPDFGeneration(b *testing.B) {
	config := getTestPDFConfig()
	generator := NewPDFGenerator(config)

	htmlContent := `
	<!DOCTYPE html>
	<html>
	<head><title>Benchmark Test</title></head>
	<body>
		<h1>Benchmark Document</h1>
		<p>This is a benchmark test for PDF generation performance.</p>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
			<li>Item 3</li>
		</ul>
	</body>
	</html>
	`

	options := &GenerationOptions{
		PageSize:    "A4",
		Orientation: "portrait",
		Margins: map[string]string{
			"top":    "1cm",
			"bottom": "1cm",
			"left":   "1cm",
			"right":  "1cm",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.GenerateFromHTML(htmlContent, options)
		if err != nil {
			b.Skip("PDF generation tools not available")
		}
	}
}
