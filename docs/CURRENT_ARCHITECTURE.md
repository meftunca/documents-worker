# 📚 Documents Worker - Current Architecture Documentation

## 🏗️ **System Architecture**

### **Architecture Pattern: Hexagonal Architecture (Ports & Adapters)**

```
┌─────────────────────────────────────────────────────────────┐
│                    External Interfaces                       │
├─────────────────────────────────────────────────────────────┤
│  CLI Interface  │  HTTP API  │  Queue Jobs  │  K8s Probes   │
├─────────────────────────────────────────────────────────────┤
│                    Primary Adapters                          │
├─────────────────────────────────────────────────────────────┤
│                      Core Domain                             │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐     │
│  │  Document   │  │  Processing  │  │     Queue       │     │
│  │  Service    │  │   Service    │  │   Service       │     │
│  └─────────────┘  └──────────────┘  └─────────────────┘     │
├─────────────────────────────────────────────────────────────┤
│                   Secondary Adapters                         │
├─────────────────────────────────────────────────────────────┤
│  Redis Queue  │  File System  │  External Tools  │  Cache   │
└─────────────────────────────────────────────────────────────┘
```

## 📁 **Directory Structure & Components**

### **Core Architecture**
```
internal/
├── core/
│   ├── domain/          # Business entities and rules
│   │   └── models.go    # Document, Job, Processing types
│   ├── ports/           # Interface definitions
│   │   ├── document.go  # Document service interface
│   │   ├── queue.go     # Queue service interface
│   │   └── health.go    # Health service interface
│   └── services/        # Business logic implementation
│       ├── document.go  # Document processing logic
│       ├── queue.go     # Queue management logic
│       └── health.go    # Health check logic
├── adapters/
│   ├── primary/         # Input adapters
│   │   ├── http/        # HTTP REST API
│   │   └── cli/         # Command line interface
│   └── secondary/       # Output adapters
│       ├── redis/       # Redis queue implementation
│       ├── filesystem/  # File storage implementation
│       └── cache/       # Caching implementation
└── infrastructure/      # Cross-cutting concerns
    ├── config/          # Configuration management
    ├── logging/         # Logging utilities
    └── metrics/         # Metrics collection
```

### **Processing Modules**
```
├── chunking/           # Document chunking for RAG
│   ├── types.go       # Chunk types and interfaces
│   └── service.go     # Modern text splitting logic
├── media/             # Media processing engines
│   ├── converter.go   # Format conversion logic
│   ├── document.go    # Document processing
│   ├── image.go       # Image processing with VIPS
│   ├── video.go       # Video processing with FFmpeg
│   └── processor_factory.go  # Processing strategy factory
├── pdfgen/            # PDF generation
│   └── generator.go   # Playwright-based PDF generation
├── ocr/               # OCR processing
│   └── ocr.go         # Tesseract integration
├── textextractor/     # Text extraction
│   └── extractor.go   # MuPDF and LibreOffice integration
└── utils/             # Utility functions
    └── file_utils.go  # MIME detection, file operations
```

## 🔧 **Current Capabilities**

### **1. Document Processing**
- **Supported Input Formats**:
  - PDF documents (.pdf)
  - Microsoft Office (.docx, .xlsx, .pptx, .doc, .xls, .ppt)
  - OpenDocument (.odt, .ods, .odp)
  - Plain text (.txt)
  - Rich text (.rtf)
  - HTML (.html, .htm)
  - Markdown (.md, .markdown)

- **Output Capabilities**:
  - PDF generation from HTML/Markdown
  - Text extraction from all document types
  - Document-to-image conversion
  - OCR text extraction from scanned documents

### **2. Image Processing**
- **Supported Formats**: JPEG, PNG, WEBP, AVIF, GIF, BMP, TIFF
- **Operations**:
  - Format conversion
  - Resize and crop
  - Quality optimization
  - Thumbnail generation
  - EXIF data handling

### **3. Video Processing**
- **Supported Formats**: MP4, AVI, MOV, MKV, WEBM
- **Operations**:
  - Format conversion
  - Thumbnail extraction
  - Basic video cutting
  - Metadata extraction

### **4. OCR Capabilities**
- **Engine**: Tesseract OCR
- **Input Types**: Images, PDFs, scanned documents
- **Languages**: Configurable language support
- **Output**: Structured text with confidence scores

### **5. Document Chunking (RAG-Ready)**
- **Methods**: Recursive, Semantic, Smart, Text-based
- **Features**:
  - HTML to Markdown conversion
  - Content cleaning for RAG
  - Configurable chunk sizes and overlap
  - Multiple output formats

## 🌐 **API Interfaces**

### **HTTP REST API**
```
Base URL: http://localhost:3001/api/v1

Document Processing:
├── POST /extract/text              # Extract text from documents
├── POST /extract/pages             # Extract text by pages
├── POST /ocr/image                 # OCR on images
├── POST /ocr/document              # OCR on documents
└── POST /sync/convert/document     # Convert documents

Media Processing:
├── POST /sync/convert/image        # Convert images
├── POST /thumbnail/image           # Generate image thumbnails
└── POST /thumbnail/video           # Generate video thumbnails

System:
├── GET  /health                    # Health check
└── GET  /queue/stats              # Queue statistics
```

### **CLI Interface**
```
Commands:
├── convert
│   ├── image                      # Image format conversion
│   ├── pdf                        # PDF generation
│   └── chunk                      # Document chunking
├── extract                        # Text extraction
├── ocr                           # OCR processing
├── thumbnail                     # Thumbnail generation
├── health                        # System health check
└── stats                         # System statistics
```

## 🔄 **Processing Architecture**

### **Synchronous Processing**
- Direct HTTP API calls
- Immediate response
- Best for small files and quick operations

### **Asynchronous Processing**
- Redis-based job queue
- Background worker processing
- Job status tracking
- Retry mechanisms for failed jobs

### **Queue System**
```
Job Flow:
1. Client submits job → Redis Queue
2. Worker picks up job → Processing
3. Result stored → Client notified
4. Cleanup and metrics update
```

## 🐳 **Deployment Architecture**

### **Docker Support**
- Multi-stage builds
- Non-root containers
- Resource limits
- Health checks

### **Kubernetes Ready**
```yaml
Resources:
├── Deployment       # Main application
├── Service          # Load balancing
├── ConfigMap        # Configuration
├── Secret           # Sensitive data
├── NetworkPolicy    # Security
└── HPA              # Auto-scaling
```

### **Dependencies**
- **Redis**: Queue and caching
- **External Tools**:
  - FFmpeg (video processing)
  - LibreOffice (office documents)
  - MuPDF/mutool (PDF processing)
  - Tesseract (OCR)
  - VIPS (image processing)
  - Playwright/Chromium (PDF generation)

## 📊 **Current Performance Characteristics**

### **File Size Limits**
- No explicit limits currently implemented
- Limited by available memory and disk space
- Recommendation: Add configurable limits

### **Concurrent Processing**
- Redis queue supports multiple workers
- Worker scaling based on queue depth
- Graceful shutdown mechanisms

### **Memory Usage**
- Document processing: Variable based on file size
- Image processing: Optimized with VIPS
- PDF generation: Playwright browser instances

## 🔍 **Monitoring & Health**

### **Health Checks**
- System dependencies (Redis, external tools)
- Resource utilization
- Queue status
- Service responsiveness

### **Metrics**
- Job processing times
- Queue statistics
- Error rates
- Resource usage

## 🧪 **Testing Coverage**

### **Unit Tests**
- Queue operations
- Worker lifecycle
- Core business logic
- Configuration handling

### **Integration Tests**
- Redis queue integration
- External tool integration
- End-to-end processing flows

### **Benchmarks**
- Performance testing for critical operations
- Memory usage profiling
- Concurrent processing tests

## 🏷️ **Current Version: v1.0**

### **Stability Level**
- ✅ **Core Features**: Production ready
- ✅ **Architecture**: Solid foundation
- ✅ **Basic Operations**: Fully functional
- ⚠️ **Advanced Features**: Some limitations
- ⚠️ **Monitoring**: Basic implementation
- ⚠️ **Error Handling**: Could be enhanced

### **Known Limitations**
1. No input validation/sanitization
2. Limited error recovery mechanisms
3. Basic logging (not structured)
4. No batch processing capabilities
5. Limited cloud storage integration
6. No webhook notifications
7. Basic monitoring/alerting

## 📈 **Performance Benchmarks** (Current)

### **Document Processing**
- Text extraction: ~2-5s for typical office documents
- PDF generation: ~3-8s for complex HTML
- OCR processing: ~5-15s depending on image quality

### **Image Processing**
- Format conversion: ~0.5-2s for typical images
- Resize operations: ~0.2-1s
- Thumbnail generation: ~0.1-0.5s

### **System Resources**
- Memory: 100-500MB baseline, varies with processing
- CPU: Scalable based on worker count
- Disk: Temporary files cleaned automatically

---

**Documentation Version**: 1.0  
**Last Updated**: August 26, 2025  
**Status**: Current Production Architecture
