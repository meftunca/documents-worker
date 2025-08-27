# Documents Worker Client Components

This directory contains client libraries and tools for interacting with the Documents Worker service.

## 📦 Components

### 1. Client Library (`./client/`)
Go package for programmatic access to the Documents Worker service.

**Features:**
- ✅ Complete API coverage (document, image, video, text, OCR, chunking)
- ✅ Synchronous and asynchronous processing
- ✅ Job management and status tracking
- ✅ Batch processing with concurrency control
- ✅ File upload and result download
- ✅ Error handling and retries
- ✅ Comprehensive test suite

**Quick Start:**
```go
import "documents-worker/cmd/client"

c := client.NewClient(client.Config{
    BaseURL: "http://localhost:8080",
})

resp, err := c.ProcessImage("photo.jpg", &client.ProcessingOptions{
    Format: "webp",
    Quality: 85,
})
```

### 2. CLI Tool (`./client-cli/`)
Command-line interface for batch operations and manual processing.

**Features:**
- ✅ All document processing operations
- ✅ Batch processing with directory scanning
- ✅ Job status monitoring and result download
- ✅ Concurrent processing control
- ✅ Verbose output and error reporting
- ✅ Multiple input/output formats

**Quick Start:**
```bash
# Build the CLI tool
cd cmd/client-cli
go build -o documents-client .

# Process an image
./documents-client -op=image -file=photo.jpg -format=webp -quality=85

# Batch process documents
./documents-client -op=document -batch=./docs -wait -concurrent=5
```

## 🚀 Getting Started

### Prerequisites
- Go 1.21+ 
- Running Documents Worker service
- Network access to the service

### Installation

#### As Go Module
```bash
go get github.com/meftunca/documents-worker/cmd/client
```

#### CLI Tool
```bash
# Build from source
git clone https://github.com/meftunca/documents-worker.git
cd documents-worker/cmd/client-cli
go build -o documents-client .

# Move to PATH (optional)
mv documents-client /usr/local/bin/
```

## 📖 Documentation

- **[Client Library README](./client/README.md)** - Comprehensive Go library documentation
- **[CLI Tool README](./client-cli/README.md)** - Command-line interface guide
- **[Usage Examples](../examples/client_usage.go)** - Practical examples

## 🔧 Configuration

### Service Connection
```bash
# Default connection
export DOCUMENTS_WORKER_URL=http://localhost:8080

# With authentication
export DOCUMENTS_WORKER_API_KEY=your-api-key

# Custom timeout
export DOCUMENTS_WORKER_TIMEOUT=60s
```

### Client Library Config
```go
config := client.Config{
    BaseURL: "http://localhost:8080",
    APIKey:  "optional-api-key",
    Timeout: 30 * time.Second,
}
```

## 🧪 Testing

### Client Library Tests
```bash
cd cmd/client
go test -v
```

### Integration Tests
```bash
# Start the service first
go run cmd/server/main.go

# Then test client connectivity
go run examples/client_usage.go
```

### CLI Tool Tests
```bash
cd cmd/client-cli
./documents-client -v  # Health check
./documents-client -h  # Help menu
```

## 📊 Supported Operations

| Operation | Library Method | CLI Flag | Description |
|-----------|---------------|----------|-------------|
| **Document** | `ProcessDocument()` | `-op=document` | Office documents to PDF |
| **Image** | `ProcessImage()` | `-op=image` | Image processing and conversion |
| **Video** | `ProcessVideo()` | `-op=video` | Video processing and conversion |
| **Text** | `ExtractText()` | `-op=text` | Text extraction from documents |
| **OCR** | `PerformOCR()` | `-op=ocr` | Optical character recognition |
| **Chunking** | `ChunkDocument()` | `-op=chunk` | Document chunking for RAG |

## 🔄 Processing Modes

### Synchronous Processing
```go
// Library
status, err := client.ProcessAndWait("file.pdf", "document", options)

// CLI
./documents-client -op=document -file=doc.pdf -wait
```

### Asynchronous Processing
```go
// Library
resp, err := client.ProcessDocument("file.pdf", options)
// Later: status, err := client.GetJobStatus(resp.JobID)

// CLI
./documents-client -op=document -file=doc.pdf
./documents-client -job=abc123  # Check status later
```

### Batch Processing
```go
// Library
statuses, err := client.BatchProcess(files, "image", options, 5)

// CLI
./documents-client -op=image -batch=./photos -concurrent=5 -wait
```

## 🎯 Use Cases

### 1. **Automated Document Pipeline**
```go
// Process incoming documents automatically
files, _ := filepath.Glob("./inbox/*.pdf")
statuses, err := client.BatchProcess(files, "document", options, 10)
```

### 2. **Image Optimization Service**
```bash
# Optimize all images in a directory
./documents-client -op=image -batch=./images -format=webp -quality=85 -concurrent=20
```

### 3. **OCR Processing Pipeline**
```go
// Extract text from scanned documents
for _, file := range scannedFiles {
    resp, _ := client.PerformOCR(file, &client.ProcessingOptions{
        Language: "tur",
    })
    // Process extracted text...
}
```

### 4. **Document Analysis**
```bash
# Extract and chunk documents for RAG
./documents-client -op=text -batch=./docs -wait
./documents-client -op=chunk -batch=./docs -wait
```

## ⚡ Performance Tips

1. **Concurrent Processing**: Use appropriate concurrency levels
   ```bash
   ./documents-client -batch=./files -concurrent=10  # Adjust based on resources
   ```

2. **Batch Operations**: Process multiple files in single operations
   ```go
   // Better than individual calls
   statuses, err := client.BatchProcess(files, "image", options, 5)
   ```

3. **Timeout Configuration**: Set appropriate timeouts for large files
   ```go
   config.Timeout = 5 * time.Minute  // For large video files
   ```

4. **Format Selection**: Choose optimal formats for your use case
   ```bash
   ./documents-client -op=image -format=webp -quality=85  # Smaller file sizes
   ```

## 🔍 Troubleshooting

### Connection Issues
```bash
# Test service connectivity
./documents-client -v

# Check service status
curl http://localhost:8080/health
```

### Authentication Issues
```bash
# Test with API key
./documents-client -key=your-api-key -v
```

### Performance Issues
```bash
# Reduce concurrency
./documents-client -batch=./files -concurrent=2

# Increase timeout
./documents-client -timeout=60s -op=video -file=large.mp4
```

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/client-improvement`
3. Add tests for new functionality
4. Submit pull request

## 📄 License

MIT License - see [LICENSE](../../LICENSE) file for details.

---

**🎯 Ready to get started?** Check out the individual README files for detailed usage instructions!
