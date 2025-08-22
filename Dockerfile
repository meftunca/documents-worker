# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o documents-worker .

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ffmpeg \
    libreoffice \
    mupdf-tools \
    tesseract-ocr \
    tesseract-ocr-data-tur \
    tesseract-ocr-data-eng \
    vips \
    vips-dev \
    ca-certificates \
    tzdata

# Create app user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Create necessary directories
RUN mkdir -p /app /tmp/documents-worker && \
    chown -R appuser:appgroup /app /tmp/documents-worker

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder --chown=appuser:appgroup /app/documents-worker .

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3001/health/liveness || exit 1

# Expose port
EXPOSE 3001

# Run the application
CMD ["./documents-worker"]
