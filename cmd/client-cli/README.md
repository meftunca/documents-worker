# Documents Worker Client CLI

Command-line interface for the Documents Worker service.

## Installation

```bash
# Build from source
cd cmd/client-cli
go build -o documents-client .

# Or install globally
go install ./cmd/client-cli
```

## Usage

```bash
documents-client [options]
```

### Basic Examples

```bash
# Health check
documents-client

# Process a single image
documents-client -op=image -file=photo.jpg -format=webp -quality=85

# Process a document and wait for completion
documents-client -op=document -file=report.pdf -wait

# Perform OCR on an image
documents-client -op=ocr -file=scan.png -lang=tur

# Extract text from a document
documents-client -op=text -file=document.docx

# Check job status
documents-client -job=abc123

# Download job result
documents-client -job=abc123 -download -output=./results

# Batch process all images in a directory
documents-client -op=image -batch=./photos -format=webp -concurrent=10 -wait
```

## Operations

### Document Processing (`-op=document`)
Process office documents (PDF, DOC, DOCX, etc.)

```bash
documents-client -op=document -file=report.docx -format=pdf -wait
```

### Image Processing (`-op=image`)
Process and convert images

```bash
documents-client -op=image -file=photo.jpg -format=webp -quality=85 -width=800 -height=600
```

### Video Processing (`-op=video`)
Process and convert videos

```bash
documents-client -op=video -file=video.mp4 -format=webm
```

### Text Extraction (`-op=text`)
Extract text from documents

```bash
documents-client -op=text -file=document.pdf -lang=tur
```

### OCR Processing (`-op=ocr`)
Perform OCR on images or scanned documents

```bash
documents-client -op=ocr -file=scan.png -lang=tur -page=1
```

### Document Chunking (`-op=chunk`)
Chunk documents for RAG applications

```bash
documents-client -op=chunk -file=document.pdf
```

## Options

### Connection Options
- `-url` - Service URL (default: http://localhost:8080)
- `-key` - API key for authentication
- `-timeout` - Request timeout (default: 30s)

### Processing Options
- `-format` - Output format (webp, pdf, png, etc.)
- `-quality` - Quality level (1-100)
- `-width` - Width in pixels
- `-height` - Height in pixels
- `-page` - Page number for PDF processing
- `-lang` - Language for OCR (default: tur)

### File Options
- `-file` - Single file to process
- `-batch` - Directory for batch processing
- `-output` - Output directory for results (default: .)

### Job Management
- `-job` - Job ID to check status or download
- `-download` - Download job result
- `-wait` - Wait for job completion

### Batch Processing
- `-concurrent` - Max concurrent jobs (default: 5)

### Other Options
- `-v` - Verbose output
- `-h` - Show help

## Examples

### Image Processing
```bash
# Convert image to WebP with specific quality
documents-client -op=image -file=photo.jpg -format=webp -quality=85

# Resize image
documents-client -op=image -file=photo.jpg -width=800 -height=600

# Batch convert all images in a directory
documents-client -op=image -batch=./photos -format=webp -quality=90 -concurrent=10 -wait
```

### Document Processing
```bash
# Convert Word document to PDF
documents-client -op=document -file=report.docx -format=pdf -wait

# Process multiple documents
documents-client -op=document -batch=./documents -wait
```

### OCR Processing
```bash
# OCR a scanned image
documents-client -op=ocr -file=scan.png -lang=tur

# OCR specific page of a PDF
documents-client -op=ocr -file=document.pdf -page=2 -lang=eng
```

### Text Extraction
```bash
# Extract text from PDF
documents-client -op=text -file=document.pdf

# Extract text from multiple files
documents-client -op=text -batch=./documents -concurrent=5 -wait
```

### Job Management
```bash
# Submit job and get job ID
documents-client -op=image -file=photo.jpg -format=webp

# Check job status
documents-client -job=abc123-def456-ghi789

# Wait for job completion and download result
documents-client -job=abc123-def456-ghi789 -download -output=./results -v
```

### With Authentication
```bash
# Use API key
documents-client -key=your-api-key -op=image -file=photo.jpg

# Use different service URL
documents-client -url=https://documents.example.com -op=document -file=report.pdf
```

## Output

### Success Output
```
✅ Job completed: abc123-def456-ghi789
💾 Result saved to: ./results/result_abc123-def456-ghi789
```

### Verbose Output
```bash
documents-client -op=image -file=photo.jpg -v
```
```
📄 Processing file: photo.jpg
🔧 Operation: image
📤 Job submitted: abc123-def456-ghi789
Response: {Success:true Message:"Image processing started" JobID:"abc123-def456-ghi789"}
```

### Batch Processing Output
```
📁 Batch processing directory: ./photos
🔧 Operation: image
⚡ Concurrent jobs: 10
📊 Found 25 files to process
✅ Batch processing completed: 25 jobs
```

## Error Handling

The CLI provides clear error messages:

```
❌ File processing failed: file not found: /path/to/file.jpg
❌ API error: Internal server error
❌ Job abc123 not found
```

## Configuration

### Environment Variables
You can set default values using environment variables:

```bash
export DOCUMENTS_WORKER_URL=http://localhost:8080
export DOCUMENTS_WORKER_API_KEY=your-api-key
export DOCUMENTS_WORKER_TIMEOUT=60s
```

### Config File Support
Create a `.documents-client.yaml` file in your home directory:

```yaml
url: http://localhost:8080
api_key: your-api-key
timeout: 30s
default_format: webp
default_quality: 85
```

## Performance Tips

1. **Batch Processing**: Use `-batch` for multiple files instead of individual calls
2. **Concurrent Jobs**: Adjust `-concurrent` based on your system and network
3. **Local Processing**: Use localhost URL for best performance
4. **Format Selection**: Choose appropriate formats for your use case

## Supported File Formats

### Input Formats
- **Documents**: PDF, DOC, DOCX, TXT, MD, HTML
- **Images**: JPG, PNG, GIF, BMP, TIFF, WEBP, AVIF
- **Videos**: MP4, AVI, MOV, WMV, FLV, WEBM

### Output Formats
- **Images**: WebP, PNG, JPG, AVIF
- **Documents**: PDF
- **Videos**: WebM, MP4

## Troubleshooting

### Connection Issues
```bash
# Test connection
documents-client -v

# Use different URL
documents-client -url=http://localhost:8081
```

### File Issues
```bash
# Check file exists
ls -la /path/to/file.jpg

# Use absolute path
documents-client -op=image -file=/absolute/path/to/file.jpg
```

### Performance Issues
```bash
# Reduce concurrent jobs
documents-client -batch=./files -concurrent=2

# Increase timeout
documents-client -timeout=60s -op=video -file=large-video.mp4
```

## Exit Codes

- `0` - Success
- `1` - General error
- `2` - File not found
- `3` - API error
- `4` - Invalid arguments

## License

MIT License
