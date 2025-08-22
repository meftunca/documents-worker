# Documents Worker

An enterprise-grade document processing microservice designed for Kubernetes clusters. This service handles various media types including documents, images, and videos with OCR capabilities and queue-based asynchronous processing.

## 🚀 Features

### Core Processing
- **Document Processing**: Office documents → PDF → Image conversion
- **Image Processing**: Resize, crop, format conversion with VIPS/FFmpeg
- **Video Processing**: Cutting, format conversion with FFmpeg
- **OCR Processing**: Text extraction from images and documents
- **Text Extraction**: Direct text extraction from documents and PDFs

### Architecture
- **Queue-based Processing**: Redis-backed job queue for scalability
- **Synchronous & Asynchronous APIs**: Backward compatibility + modern async processing
- **Health Checks**: Kubernetes-ready liveness, readiness probes
- **Monitoring**: Metrics endpoint for observability
- **Graceful Shutdown**: Clean worker termination

### Production Ready
- **Kubernetes Native**: ConfigMaps, Secrets, NetworkPolicies
- **Security**: Non-root containers, resource limits
- **Observability**: Structured logging, metrics, health checks
- **Scalability**: Horizontal pod autoscaling ready

## 📋 Prerequisites

### Runtime Dependencies
- FFmpeg
- LibreOffice
- MuPDF (mutool)
- Tesseract OCR
- VIPS (optional, for faster image processing)

### Infrastructure
- Redis (for queue management)
- Kubernetes cluster (for production deployment)

## 🛠️ Installation

### Development Setup

1. **Clone the repository**
```bash
git clone <repository-url>
cd documents-worker
```

2. **Install dependencies**
```bash
make deps
```

3. **Start development environment**
```bash
make dev
```

### Docker Setup

1. **Using Docker Compose**
```bash
make docker-run
```

2. **Check logs**
```bash
make docker-logs
```

### Kubernetes Deployment

1. **Deploy to cluster**
```bash
make k8s-deploy
```

2. **Check status**
```bash
make k8s-status
```

3. **Access logs**
```bash
make k8s-logs
```

## 🔧 Configuration

The service uses environment variables for configuration. See `config/config.go` for all available options.

### Core Settings
```bash
SERVER_PORT=3001
ENVIRONMENT=production
REDIS_HOST=redis-service
REDIS_PORT=6379
WORKER_MAX_CONCURRENCY=10
```

### External Tools
```bash
VIPS_ENABLED=true
FFMPEG_PATH=ffmpeg
LIBREOFFICE_PATH=soffice
MUTOOL_PATH=mutool
TESSERACT_PATH=tesseract
```

### OCR Settings
```bash
OCR_LANGUAGE=tur+eng
OCR_DPI=300
OCR_PSM=1
```

## 📡 API Endpoints

### Health Checks (Kubernetes)
- `GET /health` - Overall health status
- `GET /health/liveness` - Liveness probe
- `GET /health/readiness` - Readiness probe
- `GET /metrics` - Prometheus metrics

### Asynchronous Processing
- `POST /api/v1/process/document` - Queue document processing
- `POST /api/v1/process/image` - Queue image processing
- `POST /api/v1/process/video` - Queue video processing
- `GET /api/v1/job/{id}` - Check job status
- `GET /api/v1/queue/stats` - Queue statistics

### OCR Processing
- `POST /api/v1/ocr/image` - Extract text from image
- `POST /api/v1/ocr/document` - Extract text from document

### Text Extraction (Synchronous)
- `POST /api/v1/extract/text` - Extract text from any supported document
- `POST /api/v1/extract/pdf-pages` - Extract text from all PDF pages
- `POST /api/v1/extract/pdf-range` - Extract text from PDF page range

### Text Extraction (Asynchronous)
- `POST /api/v1/extract/async/text` - Queue text extraction job
- `POST /api/v1/extract/async/pdf-pages` - Queue PDF pages extraction
- `POST /api/v1/extract/async/pdf-range` - Queue PDF range extraction

### Synchronous Processing (Legacy)
- `POST /api/v1/sync/convert/document`
- `POST /api/v1/sync/convert/image`
- `POST /api/v1/sync/convert/video`

## 📝 Usage Examples

### Asynchronous Image Processing
```bash
curl -X POST http://localhost:3001/api/v1/process/image \
  -F "file=@image.jpg" \
  -F "width=800" \
  -F "height=600" \
  -F "format=webp" \
  -F "quality=90"
```

Response:
```json
{
  "job_id": "uuid-here",
  "status": "accepted",
  "message": "Job submitted for processing",
  "check_status_url": "/api/v1/job/uuid-here"
}
```

### Check Job Status
```bash
curl http://localhost:3001/api/v1/job/uuid-here
```

### OCR Processing
```bash
curl -X POST http://localhost:3001/api/v1/ocr/image \
  -F "file=@document.png"
```

Response:
```json
{
  "text": "Extracted text content...",
  "confidence": 0.95,
  "language": "tur+eng",
  "page_count": 1,
  "metadata": {
    "input_file": "document.png",
    "dpi": 300
  }
}
```

### Text Extraction

#### Extract text from document (synchronous)
```bash
curl -X POST http://localhost:3001/api/v1/extract/text \
  -F "file=@document.pdf"
```

Response:
```json
{
  "text": "Full document text content...",
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
      "pages": 5
    }
  }
}
```

#### Extract text from PDF page range
```bash
curl -X POST http://localhost:3001/api/v1/extract/pdf-range \
  -F "file=@document.pdf" \
  -F "start_page=2" \
  -F "end_page=4"
```

#### Asynchronous text extraction
```bash
curl -X POST http://localhost:3001/api/v1/extract/async/text \
  -F "file=@large-document.pdf"
```

Response:
```json
{
  "job_id": "uuid-here",
  "status": "accepted", 
  "job_type": "full",
  "message": "Text extraction job submitted for processing",
  "check_status_url": "/api/v1/job/uuid-here"
}
```

#### Check text extraction job status
```bash
curl http://localhost:3001/api/v1/job/uuid-here
```

Response:
```json
{
  "id": "uuid-here",
  "type": "text_extraction",
  "status": "completed",
  "result": {
    "extraction_result": {
      "text": "Extracted text...",
      "word_count": 1250,
      "page_count": 5
    },
    "job_type": "full",
    "processed_at": "2024-01-01T00:00:00Z"
  }
}
```

## 🔄 Queue System

The service uses Redis for job queuing with the following features:

- **Automatic Retries**: Failed jobs are retried with exponential backoff
- **Job Persistence**: Jobs are stored with 24-hour expiration
- **Status Tracking**: Real-time job status updates
- **Concurrency Control**: Configurable worker pool size

### Job Lifecycle
1. **Pending**: Job submitted to queue
2. **Processing**: Worker picked up the job
3. **Completed**: Job finished successfully
4. **Failed**: Job failed after all retries

## 📊 Monitoring

### Health Status Response
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-01T00:00:00Z",
  "uptime": "1h30m45s",
  "services": {
    "ffmpeg": {"status": "available", "available": true},
    "redis": {"status": "connected", "available": true}
  },
  "queue": {
    "connected": true,
    "stats": {"pending": 5}
  }
}
```

### Metrics
Available at `/metrics` endpoint for Prometheus scraping:
- `documents_worker_up`
- `documents_worker_uptime_seconds`
- `documents_worker_queue_pending_jobs`
- `documents_worker_service_available{service="ffmpeg"}`

## 🐳 Docker

### Build Image
```bash
make docker-build
```

### Push to Registry
```bash
make docker-push
```

### Environment Variables in Docker
```yaml
environment:
  - ENVIRONMENT=production
  - REDIS_HOST=redis
  - WORKER_MAX_CONCURRENCY=10
```

## ☸️ Kubernetes

### Resource Requirements
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

### Scaling
```bash
kubectl scale deployment documents-worker --replicas=5
```

### HPA (Horizontal Pod Autoscaler)
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: documents-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: documents-worker
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## 🔐 Security

- **Non-root containers**: Runs as user ID 1000
- **Network policies**: Restricts pod-to-pod communication
- **Resource limits**: Prevents resource exhaustion
- **Secret management**: Sensitive data via Kubernetes secrets

## 🧪 Testing

### Run Tests
```bash
make test
```

### Run with Coverage
```bash
make test-coverage
```

### Benchmark Tests
```bash
make benchmark
```

### Load Testing
```bash
# Example with hey tool
hey -n 1000 -c 10 -m POST -D image.jpg http://localhost:3001/api/v1/process/image
```

## 📈 Performance

### Typical Performance
- **Image Processing**: 100-500ms per image
- **Document Conversion**: 1-5s per document
- **OCR Processing**: 2-10s per page
- **Queue Throughput**: 100+ jobs/minute per worker

### Optimization Tips
1. **Enable VIPS**: Faster image processing
2. **Adjust Concurrency**: Based on CPU/Memory
3. **Use SSD Storage**: For temporary files
4. **Monitor Queue**: Prevent backlog buildup

## 🔧 Development

### Hot Reload
```bash
make air
```

### Debug Mode
```bash
ENVIRONMENT=development LOG_LEVEL=debug make run
```

### Profiling
```bash
make profile
```

## 📚 Future Enhancements

- [ ] **Export Capabilities**: PDF generation, ZIP archives
- [ ] **Advanced OCR**: Table extraction, form recognition
- [ ] **Batch Processing**: Multiple files in single request
- [ ] **Cloud Storage**: S3, GCS integration
- [ ] **Webhook Notifications**: Job completion callbacks
- [ ] **Rate Limiting**: API throttling
- [ ] **Audit Logging**: Processing history

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

For support and questions:

- Create an issue in the repository
- Check the troubleshooting section below
- Review the health check endpoints for service status

### Troubleshooting

**Service not starting?**
- Check dependency availability: `/health`
- Verify Redis connection
- Check resource limits

**Jobs failing?**
- Monitor `/metrics` endpoint
- Check worker logs
- Verify file permissions

**Performance issues?**
- Adjust `WORKER_MAX_CONCURRENCY`
- Monitor resource usage
- Check Redis memory usage
