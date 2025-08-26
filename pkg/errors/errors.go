package errors

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// Error types
	ValidationError    ErrorType = "validation_error"
	ProcessingError    ErrorType = "processing_error"
	InternalError      ErrorType = "internal_error"
	NotFoundError      ErrorType = "not_found_error"
	ConflictError      ErrorType = "conflict_error"
	RateLimitError     ErrorType = "rate_limit_error"
	AuthError          ErrorType = "auth_error"
	TimeoutError       ErrorType = "timeout_error"
	ResourceError      ErrorType = "resource_error"
	NetworkError       ErrorType = "network_error"
	ConfigurationError ErrorType = "configuration_error"
)

// AppError represents a structured application error
type AppError struct {
	Type       ErrorType              `json:"type"`
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	HTTPStatus int                    `json:"http_status"`
	Timestamp  time.Time              `json:"timestamp"`
	TraceID    string                 `json:"trace_id,omitempty"`
	File       string                 `json:"file,omitempty"`
	Line       int                    `json:"line,omitempty"`
	Function   string                 `json:"function,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	InnerError error                  `json:"-"` // Not serialized
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the inner error
func (e *AppError) Unwrap() error {
	return e.InnerError
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithTrace adds trace information to the error
func (e *AppError) WithTrace(traceID string) *AppError {
	e.TraceID = traceID
	return e
}

// New creates a new AppError
func New(errType ErrorType, code, message string) *AppError {
	err := &AppError{
		Type:       errType,
		Code:       code,
		Message:    message,
		HTTPStatus: getHTTPStatus(errType),
		Timestamp:  time.Now(),
	}

	// Add stack trace information
	if pc, file, line, ok := runtime.Caller(1); ok {
		err.File = file
		err.Line = line
		if fn := runtime.FuncForPC(pc); fn != nil {
			err.Function = fn.Name()
		}
	}

	return err
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errType ErrorType, code, message string) *AppError {
	appErr := New(errType, code, message)
	appErr.InnerError = err
	if err != nil {
		appErr.Details = err.Error()
	}
	return appErr
}

// Newf creates a new AppError with formatted message
func Newf(errType ErrorType, code, format string, args ...interface{}) *AppError {
	return New(errType, code, fmt.Sprintf(format, args...))
}

// Wrapf wraps an error with formatted message
func Wrapf(err error, errType ErrorType, code, format string, args ...interface{}) *AppError {
	return Wrap(err, errType, code, fmt.Sprintf(format, args...))
}

// Predefined error constructors

// NewValidationError creates a validation error
func NewValidationError(message string) *AppError {
	return New(ValidationError, "VALIDATION_FAILED", message)
}

// NewProcessingError creates a processing error
func NewProcessingError(message string) *AppError {
	return New(ProcessingError, "PROCESSING_FAILED", message)
}

// NewInternalError creates an internal error
func NewInternalError(message string) *AppError {
	return New(InternalError, "INTERNAL_ERROR", message)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string) *AppError {
	return New(NotFoundError, "NOT_FOUND", fmt.Sprintf("%s not found", resource))
}

// NewConflictError creates a conflict error
func NewConflictError(message string) *AppError {
	return New(ConflictError, "CONFLICT", message)
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(message string) *AppError {
	return New(RateLimitError, "RATE_LIMIT_EXCEEDED", message)
}

// NewAuthError creates an authentication error
func NewAuthError(message string) *AppError {
	return New(AuthError, "AUTH_FAILED", message)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string) *AppError {
	return New(TimeoutError, "TIMEOUT", fmt.Sprintf("%s operation timed out", operation))
}

// NewResourceError creates a resource error
func NewResourceError(message string) *AppError {
	return New(ResourceError, "RESOURCE_ERROR", message)
}

// NewNetworkError creates a network error
func NewNetworkError(message string) *AppError {
	return New(NetworkError, "NETWORK_ERROR", message)
}

// NewConfigurationError creates a configuration error
func NewConfigurationError(message string) *AppError {
	return New(ConfigurationError, "CONFIGURATION_ERROR", message)
}

// File processing specific errors

// NewUnsupportedFileTypeError creates an unsupported file type error
func NewUnsupportedFileTypeError(fileType string) *AppError {
	return New(ValidationError, "UNSUPPORTED_FILE_TYPE", fmt.Sprintf("File type '%s' is not supported", fileType))
}

// NewFileSizeError creates a file size error
func NewFileSizeError(size, maxSize int64) *AppError {
	return New(ValidationError, "FILE_SIZE_EXCEEDED", fmt.Sprintf("File size %d bytes exceeds maximum allowed size of %d bytes", size, maxSize))
}

// NewOCRError creates an OCR processing error
func NewOCRError(message string) *AppError {
	return New(ProcessingError, "OCR_FAILED", message)
}

// NewPDFGenerationError creates a PDF generation error
func NewPDFGenerationError(message string) *AppError {
	return New(ProcessingError, "PDF_GENERATION_FAILED", message)
}

// NewChunkingError creates a chunking error
func NewChunkingError(message string) *AppError {
	return New(ProcessingError, "CHUNKING_FAILED", message)
}

// NewQueueError creates a queue error
func NewQueueError(message string) *AppError {
	return New(ProcessingError, "QUEUE_ERROR", message)
}

// NewCacheError creates a cache error
func NewCacheError(message string) *AppError {
	return New(ProcessingError, "CACHE_ERROR", message)
}

// Error response structure for API
type ErrorResponse struct {
	Error   *AppError `json:"error"`
	Success bool      `json:"success"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(err *AppError) *ErrorResponse {
	return &ErrorResponse{
		Error:   err,
		Success: false,
	}
}

// getHTTPStatus maps error types to HTTP status codes
func getHTTPStatus(errType ErrorType) int {
	switch errType {
	case ValidationError:
		return http.StatusBadRequest
	case ProcessingError:
		return http.StatusUnprocessableEntity
	case NotFoundError:
		return http.StatusNotFound
	case ConflictError:
		return http.StatusConflict
	case RateLimitError:
		return http.StatusTooManyRequests
	case AuthError:
		return http.StatusUnauthorized
	case TimeoutError:
		return http.StatusRequestTimeout
	case ResourceError:
		return http.StatusInsufficientStorage
	case NetworkError:
		return http.StatusBadGateway
	case ConfigurationError:
		return http.StatusInternalServerError
	case InternalError:
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}

// IsType checks if the error is of a specific type
func IsType(err error, errType ErrorType) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == errType
	}
	return false
}

// IsCode checks if the error has a specific code
func IsCode(err error, code string) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetHTTPStatus returns the HTTP status code for an error
func GetHTTPStatus(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}

// Recovery middleware function
func RecoveryHandler() func() {
	return func() {
		if r := recover(); r != nil {
			var err *AppError
			switch v := r.(type) {
			case error:
				err = Wrap(v, InternalError, "PANIC_RECOVERED", "Panic recovered")
			case string:
				err = New(InternalError, "PANIC_RECOVERED", v)
			default:
				err = New(InternalError, "PANIC_RECOVERED", fmt.Sprintf("Panic recovered: %v", v))
			}

			// Add stack trace
			buf := make([]byte, 1024)
			for {
				n := runtime.Stack(buf, false)
				if n < len(buf) {
					break
				}
				buf = make([]byte, 2*len(buf))
			}
			err.WithContext("stack_trace", string(buf))
		}
	}
}

// Chain multiple errors together
type ErrorChain struct {
	errors []*AppError
}

// NewErrorChain creates a new error chain
func NewErrorChain() *ErrorChain {
	return &ErrorChain{
		errors: make([]*AppError, 0),
	}
}

// Add adds an error to the chain
func (ec *ErrorChain) Add(err *AppError) *ErrorChain {
	ec.errors = append(ec.errors, err)
	return ec
}

// HasErrors returns true if the chain has any errors
func (ec *ErrorChain) HasErrors() bool {
	return len(ec.errors) > 0
}

// Errors returns all errors in the chain
func (ec *ErrorChain) Errors() []*AppError {
	return ec.errors
}

// Error implements the error interface
func (ec *ErrorChain) Error() string {
	if len(ec.errors) == 0 {
		return ""
	}
	if len(ec.errors) == 1 {
		return ec.errors[0].Error()
	}
	return fmt.Sprintf("Multiple errors occurred: %d errors", len(ec.errors))
}

// First returns the first error in the chain
func (ec *ErrorChain) First() *AppError {
	if len(ec.errors) == 0 {
		return nil
	}
	return ec.errors[0]
}
