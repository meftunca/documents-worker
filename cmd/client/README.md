# Documents Worker Client Library

Go client library for interacting with the Documents Worker service.

## Installation

```bash
go get github.com/meftunca/documents-worker/cmd/client
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "documents-worker/cmd/client"
)

func main() {
    // Create client
    config := client.Config{
        BaseURL: "http://localhost:8080",
        APIKey:  "your-api-key", // Optional
        Timeout: 30 * time.Second,
    }
    
    c := client.NewClient(config)
    
    // Process a document
    options := &client.ProcessingOptions{
        Format:  "pdf",
        Quality: 85,
    }
    
    resp, err := c.ProcessDocument("document.docx", options)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Job submitted: %s\n", resp.JobID)
    
    // Wait for completion
    status, err := c.WaitForJob(resp.JobID, 2*time.Second)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Job completed: %s\n", status.Status)
}
```

## Available Operations

### Document Processing
```go
resp, err := client.ProcessDocument("document.pdf", &client.ProcessingOptions{
    Format: "pdf",
})
```

### Image Processing
```go
resp, err := client.ProcessImage("image.jpg", &client.ProcessingOptions{
    Format:  "webp",
    Quality: 85,
    Width:   800,
    Height:  600,
})
```

### Video Processing
```go
resp, err := client.ProcessVideo("video.mp4", &client.ProcessingOptions{
    Format: "webm",
})
```

### Text Extraction
```go
resp, err := client.ExtractText("document.pdf", &client.ProcessingOptions{
    Language: "tur",
})
```

### OCR Processing
```go
resp, err := client.PerformOCR("image.png", &client.ProcessingOptions{
    Language: "tur",
    Page:     1,
})
```

### Document Chunking
```go
resp, err := client.ChunkDocument("document.pdf", &client.ProcessingOptions{
    Metadata: map[string]string{
        "chunk_size": "1000",
        "overlap":    "200",
    },
})
```

## Job Management

### Check Job Status
```go
status, err := client.GetJobStatus("job-id-123")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s, Progress: %d%%\n", status.Status, status.Progress)
```

### Wait for Job Completion
```go
status, err := client.WaitForJob("job-id-123", 2*time.Second)
if err != nil {
    log.Fatal(err)
}

if status.Status == "completed" {
    fmt.Println("Job completed successfully!")
}
```

### Download Job Result
```go
result, err := client.GetJobResult("job-id-123")
if err != nil {
    log.Fatal(err)
}
defer result.Close()

// Save to file
file, err := os.Create("result.pdf")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

_, err = io.Copy(file, result)
if err != nil {
    log.Fatal(err)
}
```

## Batch Processing

Process multiple files concurrently:

```go
files := []string{
    "doc1.pdf",
    "doc2.docx", 
    "doc3.txt",
}

options := &client.ProcessingOptions{
    Format: "pdf",
}

statuses, err := client.BatchProcess(files, "document", options, 5) // 5 concurrent jobs
if err != nil {
    log.Fatal(err)
}

for _, status := range statuses {
    fmt.Printf("File processed: %s -> %s\n", status.ID, status.Status)
}
```

## Process and Wait

For simple synchronous processing:

```go
status, err := client.ProcessAndWait("document.pdf", "document", &client.ProcessingOptions{
    Format: "pdf",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Processing completed: %s\n", status.Status)
```

## Health Check

```go
resp, err := client.Health()
if err != nil {
    log.Fatal(err)
}

if resp.Success {
    fmt.Println("Service is healthy")
}
```

## Configuration

### Client Config
```go
config := client.Config{
    BaseURL: "http://localhost:8080",  // Required: Service URL
    APIKey:  "your-api-key",           // Optional: API key for authentication
    Timeout: 30 * time.Second,         // Optional: Request timeout (default: 30s)
}
```

### Processing Options
```go
options := &client.ProcessingOptions{
    Format:   "webp",                  // Output format
    Quality:  85,                      // Quality (1-100)
    Width:    800,                     // Width in pixels
    Height:   600,                     // Height in pixels
    Page:     1,                       // Page number for PDFs
    Language: "tur",                   // Language for OCR
    Metadata: map[string]string{       // Custom metadata
        "source": "client-library",
        "type":   "document",
    },
}
```

## Error Handling

The library returns descriptive errors:

```go
resp, err := client.ProcessDocument("nonexistent.pdf", nil)
if err != nil {
    if strings.Contains(err.Error(), "failed to open file") {
        fmt.Println("File not found")
    } else if strings.Contains(err.Error(), "API error") {
        fmt.Println("Server error")
    } else {
        fmt.Printf("Unknown error: %v\n", err)
    }
}
```

## Response Types

### Response
```go
type Response struct {
    Success bool        `json:"success"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
    JobID   string      `json:"job_id,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

### JobStatus
```go
type JobStatus struct {
    ID          string      `json:"id"`
    Status      string      `json:"status"`           // pending, processing, completed, failed
    Progress    int         `json:"progress"`         // 0-100
    Result      interface{} `json:"result,omitempty"`
    Error       string      `json:"error,omitempty"`
    CreatedAt   time.Time   `json:"created_at"`
    CompletedAt *time.Time  `json:"completed_at,omitempty"`
}
```

## Examples

See the `examples/` directory for complete examples:

- [Simple Document Processing](examples/simple_processing.go)
- [Batch Processing](examples/batch_processing.go)
- [Image Conversion](examples/image_conversion.go)
- [OCR Processing](examples/ocr_processing.go)

## Testing

```bash
go test ./cmd/client -v
```

## License

MIT License
