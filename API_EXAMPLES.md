# Documents Worker API Test Examples

## Text Extraction Examples

### 1. Extract text from any document (PDF, Office documents, etc.)
```bash
# PDF document
curl -X POST http://localhost:3001/api/v1/extract/text \
  -F "file=@sample.pdf" \
  -H "Content-Type: multipart/form-data"

# Word document  
curl -X POST http://localhost:3001/api/v1/extract/text \
  -F "file=@document.docx" \
  -H "Content-Type: multipart/form-data"

# PowerPoint document
curl -X POST http://localhost:3001/api/v1/extract/text \
  -F "file=@presentation.pptx" \
  -H "Content-Type: multipart/form-data"
```

### 2. Extract text from all PDF pages separately
```bash
curl -X POST http://localhost:3001/api/v1/extract/pdf-pages \
  -F "file=@multi-page.pdf" \
  -H "Content-Type: multipart/form-data"
```

### 3. Extract text from specific PDF page range
```bash
curl -X POST http://localhost:3001/api/v1/extract/pdf-range \
  -F "file=@document.pdf" \
  -F "start_page=2" \
  -F "end_page=5" \
  -H "Content-Type: multipart/form-data"
```

## Asynchronous Text Extraction (Queue-based)

### 1. Submit text extraction job
```bash
curl -X POST http://localhost:3001/api/v1/extract/async/text \
  -F "file=@large-document.pdf" \
  -H "Content-Type: multipart/form-data"
```

### 2. Submit PDF pages extraction job  
```bash
curl -X POST http://localhost:3001/api/v1/extract/async/pdf-pages \
  -F "file=@multi-page.pdf" \
  -H "Content-Type: multipart/form-data"
```

### 3. Submit PDF range extraction job
```bash
curl -X POST http://localhost:3001/api/v1/extract/async/pdf-range \
  -F "file=@document.pdf" \
  -F "start_page=1" \
  -F "end_page=3" \
  -H "Content-Type: multipart/form-data"
```

### 4. Check job status
```bash
# Get job status by ID (replace JOB_ID with actual job ID)
curl http://localhost:3001/api/v1/job/JOB_ID
```

### 5. Check queue statistics
```bash
curl http://localhost:3001/api/v1/queue/stats
```

## Health Checks

### 1. Overall health
```bash
curl http://localhost:3001/health
```

### 2. Liveness probe
```bash
curl http://localhost:3001/health/liveness
```

### 3. Readiness probe
```bash
curl http://localhost:3001/health/readiness
```

### 4. Metrics
```bash
curl http://localhost:3001/metrics
```

## OCR Examples

### 1. Extract text from image
```bash
curl -X POST http://localhost:3001/api/v1/ocr/image \
  -F "file=@scanned-document.png" \
  -H "Content-Type: multipart/form-data"
```

### 2. Extract text from document (OCR)
```bash
curl -X POST http://localhost:3001/api/v1/ocr/document \
  -F "file=@scanned.pdf" \
  -H "Content-Type: multipart/form-data"
```

## Media Processing (Legacy/Compatibility)

### 1. Convert image
```bash
curl -X POST http://localhost:3001/api/v1/sync/convert/image \
  -F "file=@image.jpg" \
  -F "width=800" \
  -F "height=600" \
  -F "format=webp" \
  -F "quality=90"
```

### 2. Convert document  
```bash
curl -X POST http://localhost:3001/api/v1/sync/convert/document \
  -F "file=@document.docx" \
  -F "format=pdf"
```

## Expected Response Formats

### Text Extraction Response
```json
{
  "text": "Extracted text content from the document...",
  "source_type": "pdf",
  "page_count": 5,
  "word_count": 1250,
  "char_count": 8750,
  "extracted_at": "2024-01-01T00:00:00Z",
  "duration": "2.5s",
  "metadata": {
    "source_file": "document.pdf",
    "extractor": "mutool",
    "pdf_info": {
      "title": "Sample Document",
      "author": "Author Name",
      "pages": 5
    }
  }
}
```

### PDF Pages Extraction Response
```json
{
  "pages": [
    {
      "text": "Page 1 content...",
      "source_type": "pdf_page",
      "page_count": 1,
      "word_count": 250,
      "metadata": {
        "page_number": 1
      }
    },
    {
      "text": "Page 2 content...",
      "source_type": "pdf_page", 
      "page_count": 1,
      "word_count": 300,
      "metadata": {
        "page_number": 2
      }
    }
  ],
  "total_pages": 2,
  "extracted_at": "2024-01-01T00:00:00Z"
}
```

### Async Job Response
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "accepted",
  "job_type": "full",
  "message": "Text extraction job submitted for processing",
  "check_status_url": "/api/v1/job/550e8400-e29b-41d4-a716-446655440000"
}
```

### Job Status Response
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "text_extraction",
  "status": "completed",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:05Z", 
  "completed_at": "2024-01-01T00:00:05Z",
  "result": {
    "extraction_result": {
      "text": "Extracted text...",
      "word_count": 1250,
      "page_count": 5
    },
    "job_type": "full",
    "processed_at": "2024-01-01T00:00:05Z"
  }
}
```

## Testing with Different File Types

### Supported Document Formats
- **PDF**: .pdf
- **Microsoft Office**: .docx, .xlsx, .pptx, .doc, .xls, .ppt  
- **OpenDocument**: .odt, .ods, .odp
- **Plain Text**: .txt
- **Rich Text**: .rtf

### Test Commands by File Type

```bash
# Test with different file types
curl -X POST http://localhost:3001/api/v1/extract/text -F "file=@test.pdf"
curl -X POST http://localhost:3001/api/v1/extract/text -F "file=@test.docx"
curl -X POST http://localhost:3001/api/v1/extract/text -F "file=@test.pptx"
curl -X POST http://localhost:3001/api/v1/extract/text -F "file=@test.xlsx"
curl -X POST http://localhost:3001/api/v1/extract/text -F "file=@test.odt"
curl -X POST http://localhost:3001/api/v1/extract/text -F "file=@test.txt"
```

## Error Handling Examples

### Invalid file type
```json
{
  "error": "unsupported file type: image/jpeg",
  "timestamp": "2024-01-01T00:00:00Z",
  "path": "/api/v1/extract/text"
}
```

### Invalid page range
```json
{
  "error": "Invalid page range",
  "details": "start_page and end_page must be positive and start_page <= end_page"
}
```

### File processing error
```json
{
  "error": "Text extraction failed", 
  "details": "failed to extract text with mutool: exit status 1"
}
```
