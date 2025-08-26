package chunking

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/tmc/langchaingo/textsplitter"
)

// Service implements the DocumentChunker interface
type Service struct {
	htmlConverter *md.Converter
}

// NewService creates a new chunking service
func NewService() *Service {
	// Configure HTML to Markdown converter for RAG-friendly output
	converter := md.NewConverter("", true, &md.Options{
		HorizontalRule:     "---",
		BulletListMarker:   "*",
		CodeBlockStyle:     "fenced",
		Fence:              "```",
		EmDelimiter:        "*",
		StrongDelimiter:    "**",
		LinkStyle:          "inlined",
		LinkReferenceStyle: "full",
	})

	return &Service{
		htmlConverter: converter,
	}
}

// isNavigationLink checks if text is likely a navigation link
func isNavigationLink(text string) bool {
	navPatterns := []string{
		"home", "about", "contact", "menu", "login", "signup", "register",
		"learn more", "get started", "click here", "read more", "→", "←",
		"next", "previous", "back", "continue", "skip", "close",
	}

	textLower := strings.ToLower(strings.TrimSpace(text))
	for _, pattern := range navPatterns {
		if strings.Contains(textLower, pattern) {
			return true
		}
	}
	return len(textLower) < 3 // Very short text is likely navigation
}

// ChunkDocument chunks document content
func (s *Service) ChunkDocument(ctx context.Context, content string, docType DocumentType, config ChunkConfig) (*ChunkResult, error) {
	// Preprocess content based on document type
	processedContent, err := s.preprocessContent(content, docType)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess content: %w", err)
	}

	// Clean content for RAG
	cleanContent := s.cleanContentForRAG(processedContent)

	// Create appropriate text splitter
	splitter, err := s.createTextSplitter(config, docType)
	if err != nil {
		return nil, fmt.Errorf("failed to create text splitter: %w", err)
	}

	// Split the content
	chunks, err := splitter.SplitText(cleanContent)
	if err != nil {
		return nil, fmt.Errorf("failed to split text: %w", err)
	}

	// Filter and create chunk objects
	var resultChunks []Chunk
	for i, chunk := range chunks {
		cleanChunk := strings.TrimSpace(chunk)
		if len(cleanChunk) < 10 { // Skip very small chunks
			continue
		}

		resultChunks = append(resultChunks, Chunk{
			ID:      i + 1,
			Content: cleanChunk,
			Size:    len(cleanChunk),
			Metadata: map[string]interface{}{
				"document_type": string(docType),
				"method":        string(config.Method),
			},
		})
	}

	// Calculate average size
	totalSize := len(cleanContent)
	avgSize := 0.0
	if len(resultChunks) > 0 {
		chunkSizeSum := 0
		for _, chunk := range resultChunks {
			chunkSizeSum += chunk.Size
		}
		avgSize = float64(chunkSizeSum) / float64(len(resultChunks))
	}

	return &ChunkResult{
		Chunks:       resultChunks,
		TotalChunks:  len(resultChunks),
		AverageSize:  avgSize,
		OriginalSize: totalSize,
	}, nil
}

// ChunkFromFile chunks content from file
func (s *Service) ChunkFromFile(ctx context.Context, filePath string, config ChunkConfig) (*ChunkResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Determine document type from file extension
	docType := s.determineDocumentType(filePath)

	return s.ChunkDocument(ctx, string(content), docType, config)
}

// SaveChunks saves chunks to output directory
func (s *Service) SaveChunks(ctx context.Context, result *ChunkResult, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, chunk := range result.Chunks {
		filename := fmt.Sprintf("chunk_%03d.txt", chunk.ID)
		filePath := filepath.Join(outputDir, filename)

		if err := os.WriteFile(filePath, []byte(chunk.Content), 0644); err != nil {
			return fmt.Errorf("failed to write chunk %d: %w", chunk.ID, err)
		}
	}

	return nil
}

// preprocessContent preprocesses content based on document type
func (s *Service) preprocessContent(content string, docType DocumentType) (string, error) {
	switch docType {
	case TypeHTML:
		// Convert HTML to clean Markdown
		markdown, err := s.htmlConverter.ConvertString(content)
		if err != nil {
			return "", fmt.Errorf("failed to convert HTML to Markdown: %w", err)
		}
		return markdown, nil
	case TypeMarkdown, TypeText:
		return content, nil
	case TypeOffice:
		// For office documents, content should already be extracted text
		return content, nil
	default:
		return content, nil
	}
}

// cleanContentForRAG cleans content specifically for RAG usage
func (s *Service) cleanContentForRAG(content string) string {
	// Remove excessive whitespace
	content = regexp.MustCompile(`\n\s*\n\s*\n`).ReplaceAllString(content, "\n\n")

	// Remove empty lines at start and end
	content = strings.TrimSpace(content)

	// Remove common non-content patterns
	patterns := []string{
		`!\[.*?\]\(.*?\)`,         // Images with URLs
		`\[.*?\]\(https?://.*?\)`, // External links
		`#+\s*$`,                  // Empty headers
		`^\s*[→←]+\s*$`,           // Arrow navigation
		`^\s*\.\.\.\s*$`,          // Ellipsis
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		content = re.ReplaceAllString(content, "")
	}

	// Clean up multiple consecutive empty lines
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")

	return strings.TrimSpace(content)
}

// createTextSplitter creates appropriate text splitter
func (s *Service) createTextSplitter(config ChunkConfig, docType DocumentType) (textsplitter.TextSplitter, error) {
	switch config.Method {
	case MethodRecursive:
		return textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(config.ChunkSize),
			textsplitter.WithChunkOverlap(config.Overlap),
			textsplitter.WithSeparators(s.getSeparators(docType)),
		), nil
	case MethodSemantic:
		return textsplitter.NewTokenSplitter(
			textsplitter.WithChunkSize(config.ChunkSize),
			textsplitter.WithChunkOverlap(config.Overlap),
		), nil
	case MethodSmart:
		return textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(config.ChunkSize),
			textsplitter.WithChunkOverlap(config.Overlap),
			textsplitter.WithSeparators(s.getSmartSeparators(docType)),
		), nil
	default: // MethodText
		return textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(config.ChunkSize),
			textsplitter.WithChunkOverlap(config.Overlap),
			textsplitter.WithSeparators([]string{"\n\n", "\n", " ", ""}),
		), nil
	}
}

// getSeparators returns separators for document type
func (s *Service) getSeparators(docType DocumentType) []string {
	switch docType {
	case TypeMarkdown:
		return []string{"\n## ", "\n# ", "\n### ", "\n\n", "\n", ". ", " ", ""}
	case TypeHTML:
		return []string{"\n## ", "\n# ", "\n### ", "\n\n", "\n", ". ", " ", ""}
	default:
		return []string{"\n\n", "\n", ". ", " ", ""}
	}
}

// getSmartSeparators returns optimized separators for RAG
func (s *Service) getSmartSeparators(docType DocumentType) []string {
	switch docType {
	case TypeMarkdown, TypeHTML:
		return []string{
			"\n# ", "\n## ", "\n### ", "\n#### ", // Headers
			"\n\n",           // Paragraphs
			". ", "! ", "? ", // Sentences
			"\n", " ", "", // Fallbacks
		}
	default:
		return []string{"\n\n", ". ", "! ", "? ", "\n", " ", ""}
	}
}

// determineDocumentType determines document type from file path
func (s *Service) determineDocumentType(filePath string) DocumentType {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".html", ".htm":
		return TypeHTML
	case ".md", ".markdown":
		return TypeMarkdown
	case ".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt":
		return TypeOffice
	default:
		return TypeText
	}
}
