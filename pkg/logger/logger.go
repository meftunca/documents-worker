package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ContextKey is used to store correlation IDs in context
type ContextKey string

const (
	CorrelationIDKey ContextKey = "correlation_id"
	RequestIDKey     ContextKey = "request_id"
	UserIDKey        ContextKey = "user_id"
)

// Logger wraps zerolog with additional functionality
type Logger struct {
	*zerolog.Logger
}

// Config holds logger configuration
type Config struct {
	Level      string `json:"level" validate:"oneof=trace debug info warn error fatal panic"`
	Format     string `json:"format" validate:"oneof=json console"`
	Output     string `json:"output" validate:"oneof=stdout stderr file"`
	Filename   string `json:"filename,omitempty"`
	TimeFormat string `json:"time_format"`
}

// DefaultConfig returns a default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		TimeFormat: time.RFC3339,
	}
}

// New creates a new structured logger
func New(config *Config) (*Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Set log level
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}
	zerolog.SetGlobalLevel(level)

	// Configure time format
	zerolog.TimeFieldFormat = config.TimeFormat

	// Configure output
	var output io.Writer
	switch config.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	case "file":
		if config.Filename == "" {
			config.Filename = "logs/app.log"
		}
		file, err := os.OpenFile(config.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		output = file
	default:
		output = os.Stdout
	}

	// Configure format
	var logger zerolog.Logger
	if config.Format == "console" {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: output, TimeFormat: time.RFC3339}).
			With().
			Timestamp().
			Caller().
			Logger()
	} else {
		logger = zerolog.New(output).
			With().
			Timestamp().
			Caller().
			Logger()
	}

	return &Logger{Logger: &logger}, nil
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context) context.Context {
	correlationID := uuid.New().String()
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// FromContext creates a logger with context values
func (l *Logger) FromContext(ctx context.Context) *zerolog.Logger {
	logger := l.Logger.With()

	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		logger = logger.Str("correlation_id", correlationID)
	}

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		logger = logger.Str("request_id", requestID)
	}

	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		logger = logger.Str("user_id", userID)
	}

	contextLogger := logger.Logger()
	return &contextLogger
}

// LogRequest logs HTTP request details
func (l *Logger) LogRequest(ctx context.Context, method, path, userAgent, clientIP string, duration time.Duration) {
	l.FromContext(ctx).Info().
		Str("method", method).
		Str("path", path).
		Str("user_agent", userAgent).
		Str("client_ip", clientIP).
		Dur("duration", duration).
		Msg("HTTP request processed")
}

// LogError logs error with context
func (l *Logger) LogError(ctx context.Context, err error, msg string, fields map[string]interface{}) {
	event := l.FromContext(ctx).Error().Err(err)
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	event.Msg(msg)
}

// LogProcessingStart logs when document processing starts
func (l *Logger) LogProcessingStart(ctx context.Context, filename, fileType string, fileSize int64) {
	l.FromContext(ctx).Info().
		Str("filename", filename).
		Str("file_type", fileType).
		Int64("file_size", fileSize).
		Msg("Document processing started")
}

// LogProcessingComplete logs when document processing completes
func (l *Logger) LogProcessingComplete(ctx context.Context, filename string, duration time.Duration, outputSize int64) {
	l.FromContext(ctx).Info().
		Str("filename", filename).
		Dur("processing_duration", duration).
		Int64("output_size", outputSize).
		Msg("Document processing completed")
}

// LogQueueOperation logs queue operations
func (l *Logger) LogQueueOperation(ctx context.Context, operation, queueName string, messageCount int) {
	l.FromContext(ctx).Info().
		Str("operation", operation).
		Str("queue_name", queueName).
		Int("message_count", messageCount).
		Msg("Queue operation")
}

// LogPerformanceMetric logs performance metrics
func (l *Logger) LogPerformanceMetric(ctx context.Context, metricName string, value float64, unit string) {
	l.FromContext(ctx).Info().
		Str("metric_name", metricName).
		Float64("value", value).
		Str("unit", unit).
		Msg("Performance metric")
}

// Global logger instance
var globalLogger *Logger

// Init initializes the global logger
func Init(config *Config) error {
	logger, err := New(config)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// Get returns the global logger
func Get() *Logger {
	if globalLogger == nil {
		// Fallback to default logger
		logger, _ := New(DefaultConfig())
		globalLogger = logger
	}
	return globalLogger
}

// Info logs an info message
func Info() *zerolog.Event {
	return log.Info()
}

// Error logs an error message
func Error() *zerolog.Event {
	return log.Error()
}

// Debug logs a debug message
func Debug() *zerolog.Event {
	return log.Debug()
}

// Warn logs a warning message
func Warn() *zerolog.Event {
	return log.Warn()
}

// Fatal logs a fatal message and exits
func Fatal() *zerolog.Event {
	return log.Fatal()
}
