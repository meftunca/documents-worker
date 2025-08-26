package chunking

import "context"

// ChunkMethod defines the chunking strategy
type ChunkMethod string

const (
	MethodRecursive ChunkMethod = "recursive"
	MethodSemantic  ChunkMethod = "semantic"
	MethodSmart     ChunkMethod = "smart"
	MethodText      ChunkMethod = "text"
)

// DocumentType defines the input document type
type DocumentType string

const (
	TypeHTML     DocumentType = "html"
	TypeMarkdown DocumentType = "markdown"
	TypeOffice   DocumentType = "office"
	TypeText     DocumentType = "text"
)

// ChunkConfig holds configuration for chunking
type ChunkConfig struct {
	Method             ChunkMethod
	ChunkSize          int
	Overlap            int
	OutputFormat       string
	PreserveFormatting bool
}

// Chunk represents a single document chunk
type Chunk struct {
	ID       int                    `json:"id"`
	Content  string                 `json:"content"`
	Size     int                    `json:"size"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ChunkResult holds the chunking result
type ChunkResult struct {
	Chunks       []Chunk `json:"chunks"`
	TotalChunks  int     `json:"total_chunks"`
	AverageSize  float64 `json:"average_size"`
	OriginalSize int     `json:"original_size"`
}

// DocumentChunker interface for document chunking
type DocumentChunker interface {
	ChunkDocument(ctx context.Context, content string, docType DocumentType, config ChunkConfig) (*ChunkResult, error)
	ChunkFromFile(ctx context.Context, filePath string, config ChunkConfig) (*ChunkResult, error)
	SaveChunks(ctx context.Context, result *ChunkResult, outputDir string) error
}
