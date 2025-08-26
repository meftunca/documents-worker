package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all application metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal    prometheus.CounterVec
	HTTPRequestDuration  prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge
	HTTPResponseSize     prometheus.HistogramVec

	// Document processing metrics
	DocumentsProcessedTotal    prometheus.CounterVec
	DocumentProcessingDuration prometheus.HistogramVec
	DocumentProcessingErrors   prometheus.CounterVec
	DocumentSizeBytes          prometheus.HistogramVec

	// Queue metrics
	QueueSize                prometheus.GaugeVec
	QueueProcessingDuration  prometheus.HistogramVec
	QueueItemsProcessedTotal prometheus.CounterVec
	QueueItemsFailedTotal    prometheus.CounterVec

	// System metrics
	ActiveWorkers    prometheus.Gauge
	MemoryUsageBytes prometheus.Gauge
	DiskUsageBytes   prometheus.GaugeVec
	CacheHitRatio    prometheus.Gauge

	// OCR specific metrics
	OCRProcessingDuration  prometheus.HistogramVec
	OCRAccuracyScore       prometheus.HistogramVec
	OCRCharactersExtracted prometheus.CounterVec

	// Chunking metrics
	ChunksCreatedTotal prometheus.CounterVec
	ChunkSizeBytes     prometheus.HistogramVec
	ChunkingDuration   prometheus.HistogramVec
}

// New creates a new metrics instance
func New(namespace, subsystem string) *Metrics {
	return &Metrics{
		// HTTP metrics
		HTTPRequestsTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),

		HTTPRequestDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_request_duration_seconds",
				Help:      "Duration of HTTP requests in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),

		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_requests_in_flight",
				Help:      "Current number of HTTP requests being processed",
			},
		),

		HTTPResponseSize: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_response_size_bytes",
				Help:      "Size of HTTP responses in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "endpoint"},
		),

		// Document processing metrics
		DocumentsProcessedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "documents_processed_total",
				Help:      "Total number of documents processed",
			},
			[]string{"type", "status"},
		),

		DocumentProcessingDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "document_processing_duration_seconds",
				Help:      "Duration of document processing in seconds",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
			},
			[]string{"type", "operation"},
		),

		DocumentProcessingErrors: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "document_processing_errors_total",
				Help:      "Total number of document processing errors",
			},
			[]string{"type", "error_type"},
		),

		DocumentSizeBytes: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "document_size_bytes",
				Help:      "Size of processed documents in bytes",
				Buckets:   prometheus.ExponentialBuckets(1024, 2, 20),
			},
			[]string{"type"},
		),

		// Queue metrics
		QueueSize: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "queue_size",
				Help:      "Current size of processing queues",
			},
			[]string{"queue_name"},
		),

		QueueProcessingDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "queue_processing_duration_seconds",
				Help:      "Duration of queue item processing in seconds",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"queue_name"},
		),

		QueueItemsProcessedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "queue_items_processed_total",
				Help:      "Total number of queue items processed",
			},
			[]string{"queue_name", "status"},
		),

		QueueItemsFailedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "queue_items_failed_total",
				Help:      "Total number of failed queue items",
			},
			[]string{"queue_name", "error_type"},
		),

		// System metrics
		ActiveWorkers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "active_workers",
				Help:      "Current number of active workers",
			},
		),

		MemoryUsageBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_usage_bytes",
				Help:      "Current memory usage in bytes",
			},
		),

		DiskUsageBytes: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "disk_usage_bytes",
				Help:      "Current disk usage in bytes",
			},
			[]string{"path"},
		),

		CacheHitRatio: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "cache_hit_ratio",
				Help:      "Cache hit ratio (0-1)",
			},
		),

		// OCR specific metrics
		OCRProcessingDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "ocr_processing_duration_seconds",
				Help:      "Duration of OCR processing in seconds",
				Buckets:   []float64{0.5, 1, 2, 5, 10, 20, 30, 60},
			},
			[]string{"engine"},
		),

		OCRAccuracyScore: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "ocr_accuracy_score",
				Help:      "OCR accuracy score (0-1)",
				Buckets:   []float64{0.5, 0.6, 0.7, 0.8, 0.85, 0.9, 0.95, 0.98, 1.0},
			},
			[]string{"engine"},
		),

		OCRCharactersExtracted: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "ocr_characters_extracted_total",
				Help:      "Total number of characters extracted by OCR",
			},
			[]string{"engine"},
		),

		// Chunking metrics
		ChunksCreatedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "chunks_created_total",
				Help:      "Total number of chunks created",
			},
			[]string{"strategy", "document_type"},
		),

		ChunkSizeBytes: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "chunk_size_bytes",
				Help:      "Size of created chunks in bytes",
				Buckets:   []float64{100, 500, 1000, 2000, 4000, 8000, 16000},
			},
			[]string{"strategy"},
		),

		ChunkingDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "chunking_duration_seconds",
				Help:      "Duration of chunking process in seconds",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
			},
			[]string{"strategy"},
		),
	}
}

// RecordHTTPRequest records HTTP request metrics
func (m *Metrics) RecordHTTPRequest(method, endpoint, statusCode string, duration time.Duration, responseSize int64) {
	m.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	m.HTTPResponseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
}

// RecordDocumentProcessing records document processing metrics
func (m *Metrics) RecordDocumentProcessing(docType, operation, status string, duration time.Duration, sizeBytes int64) {
	m.DocumentsProcessedTotal.WithLabelValues(docType, status).Inc()
	m.DocumentProcessingDuration.WithLabelValues(docType, operation).Observe(duration.Seconds())
	m.DocumentSizeBytes.WithLabelValues(docType).Observe(float64(sizeBytes))
}

// RecordDocumentError records document processing error
func (m *Metrics) RecordDocumentError(docType, errorType string) {
	m.DocumentProcessingErrors.WithLabelValues(docType, errorType).Inc()
}

// RecordQueueOperation records queue operation metrics
func (m *Metrics) RecordQueueOperation(queueName, status string, duration time.Duration) {
	m.QueueItemsProcessedTotal.WithLabelValues(queueName, status).Inc()
	m.QueueProcessingDuration.WithLabelValues(queueName).Observe(duration.Seconds())
}

// RecordQueueError records queue processing error
func (m *Metrics) RecordQueueError(queueName, errorType string) {
	m.QueueItemsFailedTotal.WithLabelValues(queueName, errorType).Inc()
}

// SetQueueSize sets current queue size
func (m *Metrics) SetQueueSize(queueName string, size float64) {
	m.QueueSize.WithLabelValues(queueName).Set(size)
}

// SetActiveWorkers sets the number of active workers
func (m *Metrics) SetActiveWorkers(count float64) {
	m.ActiveWorkers.Set(count)
}

// SetMemoryUsage sets current memory usage
func (m *Metrics) SetMemoryUsage(bytes float64) {
	m.MemoryUsageBytes.Set(bytes)
}

// SetDiskUsage sets current disk usage
func (m *Metrics) SetDiskUsage(path string, bytes float64) {
	m.DiskUsageBytes.WithLabelValues(path).Set(bytes)
}

// SetCacheHitRatio sets cache hit ratio
func (m *Metrics) SetCacheHitRatio(ratio float64) {
	m.CacheHitRatio.Set(ratio)
}

// RecordOCRProcessing records OCR processing metrics
func (m *Metrics) RecordOCRProcessing(engine string, duration time.Duration, accuracyScore float64, charactersExtracted int64) {
	m.OCRProcessingDuration.WithLabelValues(engine).Observe(duration.Seconds())
	m.OCRAccuracyScore.WithLabelValues(engine).Observe(accuracyScore)
	m.OCRCharactersExtracted.WithLabelValues(engine).Add(float64(charactersExtracted))
}

// RecordChunking records chunking metrics
func (m *Metrics) RecordChunking(strategy, docType string, duration time.Duration, chunkCount int, avgChunkSize float64) {
	m.ChunksCreatedTotal.WithLabelValues(strategy, docType).Add(float64(chunkCount))
	m.ChunkingDuration.WithLabelValues(strategy).Observe(duration.Seconds())
	m.ChunkSizeBytes.WithLabelValues(strategy).Observe(avgChunkSize)
}

// Global metrics instance
var globalMetrics *Metrics

// Init initializes global metrics
func Init(namespace, subsystem string) {
	globalMetrics = New(namespace, subsystem)
}

// Get returns the global metrics instance
func Get() *Metrics {
	if globalMetrics == nil {
		globalMetrics = New("documents", "worker")
	}
	return globalMetrics
}
