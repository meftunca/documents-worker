# Documents Worker - Hexagonal Architecture

A high-performance document processing service built with **Hexagonal Architecture** (Ports & Adapters pattern) in Go. Supports document conversion, OCR processing, text extraction, and PDF generation through both HTTP API and CLI interfaces.

## 🏗️ Architecture

This project follows **Hexagonal Architecture** principles with clean separation of concerns:

```
internal/
├── core/                    # Business Logic Layer
│   ├── domain/             # Domain Models & Entities
│   ├── ports/              # Interface Definitions (Contracts)
│   └── services/           # Business Logic Implementation
└── adapters/               # External World Adapters
    ├── primary/            # Inbound Adapters (API/CLI)
    │   ├── http/          # REST API Handler
    │   └── cli/           # Command Line Interface
    └── secondary/         # Outbound Adapters (Infrastructure)
        └── processors/    # Document Processing Adapters
```

### Key Benefits
- ✅ **Testable**: Business logic isolated from infrastructure
- ✅ **Flexible**: Easy to swap implementations
- ✅ **Maintainable**: Clear separation of concerns
- ✅ **Scalable**: Plugin-based architecture

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Redis (for queue management)
- VIPS (for image processing)
- FFmpeg (for video processing) 
- Tesseract (for OCR)
- Playwright (for PDF generation)

### Installation

```bash
# Clone repository
git clone <your-repo-url>
cd documents-worker

# Install dependencies
make install-deps

# Build all binaries
make build
```

## 🎯 Usage

### HTTP Server

Start the HTTP server:
```bash
# Development mode
make dev

# Production build and run
make server
```

The server will start on `http://localhost:8080` with the following endpoints:

- `POST /api/v1/process` - Process documents
- `POST /api/v1/convert` - Convert between formats
- `POST /api/v1/ocr` - OCR text extraction
- `GET /api/v1/health` - Health check
- `GET /api/v1/queue/stats` - Queue statistics

### CLI Tool

The CLI provides direct document processing capabilities:

```bash
# Build CLI
make build-cli

# Convert image formats
make cli ARGS='convert input.jpg output.png'

# OCR processing
make cli ARGS='ocr image.png output.txt -l eng'

# Extract text from documents  
make cli ARGS='extract document.pdf output.txt'

# Generate thumbnails
make cli ARGS='thumbnail document.pdf thumb.jpg -w 200 -h 200'

# Health check
make cli ARGS='health'

# Queue statistics
make cli ARGS='stats'

# Show version
make cli ARGS='version'
```

## 📂 Project Structure

```
.
├── cmd/                    # Application Entry Points
│   ├── server/            # HTTP Server Entry Point
│   └── cli/               # CLI Entry Point
├── internal/              # Private Application Code
│   ├── core/             # Business Logic (Domain Layer)
│   │   ├── domain/       # Business Entities
│   │   ├── ports/        # Interface Contracts
│   │   └── services/     # Business Logic Implementation  
│   └── adapters/         # Infrastructure Layer
│       ├── primary/      # Inbound Adapters
│       │   ├── http/    # REST API
│       │   └── cli/     # Command Line Interface
│       └── secondary/   # Outbound Adapters
│           └── processors/ # Document Processing
├── config/               # Configuration Management
├── cache/                # Caching Layer (Legacy)
├── queue/                # Queue Management (Legacy)
├── health/               # Health Checking (Legacy)
└── test_files/          # Sample Files for Testing
```

## 🔧 Features

### Document Processing
- **Image Conversion**: JPEG, PNG, WebP, AVIF, TIFF
- **Video Processing**: MP4, WebM, AVI format conversion
- **PDF Generation**: HTML to PDF using Playwright
- **OCR Processing**: Text extraction from images
- **Text Extraction**: Extract text from various document formats

### Architecture Features
- **Hexagonal Architecture**: Clean separation of business logic
- **Dependency Injection**: Loose coupling between components
- **Interface Segregation**: Small, focused interfaces
- **Repository Pattern**: Data access abstraction
- **Event-Driven**: Asynchronous processing support

### Infrastructure
- **Redis Queue**: Background job processing
- **Caching**: Performance optimization
- **Health Monitoring**: System status tracking
- **Metrics**: Performance and usage statistics

## 🧪 Testing

```bash
# Run all tests
make test

# Run unit tests only  
make test-unit

# Run integration tests
make test-integration

# Run benchmarks
make benchmark
```

## 📊 Monitoring

### Health Check
```bash
# CLI health check
make cli ARGS='health'

# HTTP health check
curl http://localhost:8080/health
```

### Queue Statistics
```bash
# CLI stats
make cli ARGS='stats'

# HTTP stats
curl http://localhost:8080/api/v1/queue/stats
```

## 🐳 Docker Support

```bash
# Build Docker image
make docker-build

# Run with Docker Compose
make docker-run

# Start only services (Redis, etc.)
make services-up

# View logs
make docker-logs

# Stop services
make services-down
```

## ⚙️ Configuration

Configuration is managed through environment variables and config files:

```yaml
# config.yaml
server:
  port: "8080"
  environment: "development"

redis:
  host: "localhost:6379"
  password: ""
  db: 0

cache:
  ttl: "1h"

worker:
  num_workers: 4
  queue_size: 1000

external:
  tesseract_path: "/usr/bin/tesseract"
  playwright_path: "/usr/bin/playwright"
```

## 🚀 Development

### Adding New Processors

1. Define interface in `internal/core/ports/ports.go`
2. Implement adapter in `internal/adapters/secondary/processors/`
3. Register in dependency injection container
4. Add tests

### Adding New Endpoints

1. Add method to service interface in `internal/core/ports/`
2. Implement business logic in `internal/core/services/`
3. Add HTTP handler in `internal/adapters/primary/http/`
4. Add CLI command in `internal/adapters/primary/cli/`

## 📋 Available Commands

```bash
make help                    # Show all available commands
make build                   # Build all binaries
make build-server           # Build server only
make build-cli              # Build CLI only  
make server                 # Run HTTP server
make cli ARGS="..."         # Run CLI with arguments
make dev                    # Development mode
make test                   # Run tests with coverage
make clean                  # Clean build artifacts
make fmt                    # Format code
make lint                   # Lint code
make examples               # Show CLI examples
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Follow hexagonal architecture principles
4. Add tests for new functionality
5. Submit a pull request

## 📄 License

[MIT License](LICENSE)

## 🙏 Acknowledgments

- **Hexagonal Architecture** by Alistair Cockburn
- **Clean Architecture** by Robert C. Martin
- **Domain-Driven Design** by Eric Evans
