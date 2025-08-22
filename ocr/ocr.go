package ocr

import (
	"documents-worker/config"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type OCRResult struct {
	Text       string                 `json:"text"`
	Confidence float64                `json:"confidence"`
	Language   string                 `json:"language"`
	PageCount  int                    `json:"page_count"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type OCRProcessor struct {
	config   *config.OCRConfig
	external *config.ExternalConfig
}

func NewOCRProcessor(ocrConfig *config.OCRConfig, externalConfig *config.ExternalConfig) *OCRProcessor {
	return &OCRProcessor{
		config:   ocrConfig,
		external: externalConfig,
	}
}

func (o *OCRProcessor) ProcessImage(imagePath string) (*OCRResult, error) {
	// Create temporary output file for text
	outputFile, err := os.CreateTemp("", "ocr-output-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(outputFile.Name())
	defer outputFile.Close()

	// Build tesseract command
	args := []string{
		imagePath,
		strings.TrimSuffix(outputFile.Name(), ".txt"), // tesseract adds .txt automatically
		"-l", o.config.Language,
		"--psm", fmt.Sprintf("%d", o.config.PSM),
		"-c", "tessedit_char_whitelist=abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ğüşıöçĞÜŞİÖÇ .,!?:;()-",
	}

	cmd := exec.Command(o.external.TesseractPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tesseract execution failed: %w, output: %s", err, string(output))
	}

	// Read extracted text
	textBytes, err := os.ReadFile(outputFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read OCR output: %w", err)
	}

	text := strings.TrimSpace(string(textBytes))

	// Calculate basic confidence (simplified)
	confidence := o.calculateConfidence(text)

	return &OCRResult{
		Text:       text,
		Confidence: confidence,
		Language:   o.config.Language,
		PageCount:  1,
		Metadata: map[string]interface{}{
			"input_file": filepath.Base(imagePath),
			"psm":        o.config.PSM,
			"dpi":        o.config.DPI,
		},
	}, nil
}

func (o *OCRProcessor) ProcessPDF(pdfPath string, pageNum int) (*OCRResult, error) {
	// First convert PDF page to image
	imagePath, err := o.convertPDFPageToImage(pdfPath, pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to convert PDF to image: %w", err)
	}
	defer os.Remove(imagePath)

	// Process the image
	result, err := o.ProcessImage(imagePath)
	if err != nil {
		return nil, err
	}

	// Update metadata
	result.Metadata["source_type"] = "pdf"
	result.Metadata["page_number"] = pageNum
	result.Metadata["source_file"] = filepath.Base(pdfPath)

	return result, nil
}

func (o *OCRProcessor) ProcessDocument(docPath string) (*OCRResult, error) {
	// For documents, we'll first convert to PDF, then to image, then OCR
	// This is a simplified approach - in production you might want to extract text directly

	// Convert document to PDF first (if it's not already PDF)
	pdfPath := docPath
	ext := strings.ToLower(filepath.Ext(docPath))

	if ext != ".pdf" {
		convertedPDF, err := o.convertDocumentToPDF(docPath)
		if err != nil {
			return nil, fmt.Errorf("failed to convert document to PDF: %w", err)
		}
		defer os.Remove(convertedPDF)
		pdfPath = convertedPDF
	}

	// Process first page of PDF
	return o.ProcessPDF(pdfPath, 1)
}

func (o *OCRProcessor) convertPDFPageToImage(pdfPath string, pageNum int) (string, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("pdf-page-%d.png", pageNum))

	cmd := exec.Command("mutool", "draw",
		"-o", outputPath,
		"-r", fmt.Sprintf("%d", o.config.DPI),
		pdfPath,
		fmt.Sprintf("%d", pageNum),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("mutool execution failed: %w, output: %s", err, string(output))
	}

	return outputPath, nil
}

func (o *OCRProcessor) convertDocumentToPDF(docPath string) (string, error) {
	outputDir := os.TempDir()

	cmd := exec.Command("soffice",
		"--headless",
		"--convert-to", "pdf",
		docPath,
		"--outdir", outputDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("libreoffice execution failed: %w, output: %s", err, string(output))
	}

	// Construct expected output path
	filename := strings.TrimSuffix(filepath.Base(docPath), filepath.Ext(docPath))
	pdfPath := filepath.Join(outputDir, filename+".pdf")

	return pdfPath, nil
}

func (o *OCRProcessor) calculateConfidence(text string) float64 {
	// Simplified confidence calculation
	// In a real implementation, you'd use tesseract's confidence output

	if len(text) == 0 {
		return 0.0
	}

	// Basic heuristics:
	// - Longer text usually means better OCR
	// - Presence of common words increases confidence
	// - Too many special characters decreases confidence

	confidence := 0.5 // Base confidence

	// Length bonus
	if len(text) > 100 {
		confidence += 0.2
	} else if len(text) > 50 {
		confidence += 0.1
	}

	// Common Turkish/English words bonus
	commonWords := []string{"the", "and", "or", "bir", "ve", "bu", "da", "de", "için", "ile"}
	lowerText := strings.ToLower(text)

	for _, word := range commonWords {
		if strings.Contains(lowerText, word) {
			confidence += 0.05
		}
	}

	// Too many special characters penalty
	specialChars := "!@#$%^&*()_+=[]{}|;:,.<>?"
	specialCount := 0
	for _, char := range text {
		if strings.ContainsRune(specialChars, char) {
			specialCount++
		}
	}

	if float64(specialCount)/float64(len(text)) > 0.1 {
		confidence -= 0.2
	}

	// Ensure confidence is between 0 and 1
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// BatchProcessPDF processes all pages of a PDF
func (o *OCRProcessor) BatchProcessPDF(pdfPath string) ([]*OCRResult, error) {
	// Get page count first
	pageCount, err := o.getPDFPageCount(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF page count: %w", err)
	}

	var results []*OCRResult
	for i := 1; i <= pageCount; i++ {
		result, err := o.ProcessPDF(pdfPath, i)
		if err != nil {
			// Log error but continue with other pages
			fmt.Printf("Failed to process page %d: %v\n", i, err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

func (o *OCRProcessor) getPDFPageCount(pdfPath string) (int, error) {
	cmd := exec.Command("mutool", "info", pdfPath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get PDF info: %w", err)
	}

	// Parse output to get page count
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Pages:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var pageCount int
				if _, err := fmt.Sscanf(parts[1], "%d", &pageCount); err == nil {
					return pageCount, nil
				}
			}
		}
	}

	return 1, nil // Default to 1 page if can't determine
}
