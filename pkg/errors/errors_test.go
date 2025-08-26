package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError(t *testing.T) {
	t.Run("create new error", func(t *testing.T) {
		err := New(ValidationError, "TEST_ERROR", "This is a test error")

		assert.Equal(t, ValidationError, err.Type)
		assert.Equal(t, "TEST_ERROR", err.Code)
		assert.Equal(t, "This is a test error", err.Message)
		assert.Equal(t, 400, err.HTTPStatus) // ValidationError maps to 400
		assert.NotZero(t, err.Timestamp)
		assert.NotEmpty(t, err.File)
		assert.NotZero(t, err.Line)
	})

	t.Run("wrap existing error", func(t *testing.T) {
		originalErr := fmt.Errorf("original error")
		wrappedErr := Wrap(originalErr, ProcessingError, "WRAP_ERROR", "Wrapped error")

		assert.Equal(t, ProcessingError, wrappedErr.Type)
		assert.Equal(t, "WRAP_ERROR", wrappedErr.Code)
		assert.Equal(t, "Wrapped error", wrappedErr.Message)
		assert.Equal(t, "original error", wrappedErr.Details)
		assert.Equal(t, originalErr, wrappedErr.InnerError)
		assert.Equal(t, 422, wrappedErr.HTTPStatus) // ProcessingError maps to 422
	})

	t.Run("error with context", func(t *testing.T) {
		err := New(InternalError, "CONTEXT_ERROR", "Error with context").
			WithContext("user_id", "123").
			WithContext("operation", "file_upload").
			WithTrace("trace-123")

		assert.Equal(t, "123", err.Context["user_id"])
		assert.Equal(t, "file_upload", err.Context["operation"])
		assert.Equal(t, "trace-123", err.TraceID)
	})
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name               string
		constructor        func(string) *AppError
		expectedType       ErrorType
		expectedHTTPStatus int
	}{
		{
			name:               "validation error",
			constructor:        NewValidationError,
			expectedType:       ValidationError,
			expectedHTTPStatus: 400,
		},
		{
			name:               "processing error",
			constructor:        NewProcessingError,
			expectedType:       ProcessingError,
			expectedHTTPStatus: 422,
		},
		{
			name:               "internal error",
			constructor:        NewInternalError,
			expectedType:       InternalError,
			expectedHTTPStatus: 500,
		},
		{
			name:               "rate limit error",
			constructor:        NewRateLimitError,
			expectedType:       RateLimitError,
			expectedHTTPStatus: 429,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor("test message")
			assert.Equal(t, tt.expectedType, err.Type)
			assert.Equal(t, tt.expectedHTTPStatus, err.HTTPStatus)
			assert.Equal(t, "test message", err.Message)
		})
	}
}

func TestSpecificErrors(t *testing.T) {
	t.Run("unsupported file type error", func(t *testing.T) {
		err := NewUnsupportedFileTypeError("exe")
		assert.Equal(t, ValidationError, err.Type)
		assert.Equal(t, "UNSUPPORTED_FILE_TYPE", err.Code)
		assert.Contains(t, err.Message, "exe")
	})

	t.Run("file size error", func(t *testing.T) {
		err := NewFileSizeError(200*1024*1024, 100*1024*1024) // 200MB > 100MB
		assert.Equal(t, ValidationError, err.Type)
		assert.Equal(t, "FILE_SIZE_EXCEEDED", err.Code)
		assert.Contains(t, err.Message, "209715200") // 200MB in bytes
		assert.Contains(t, err.Message, "104857600") // 100MB in bytes
	})

	t.Run("OCR error", func(t *testing.T) {
		err := NewOCRError("OCR processing failed")
		assert.Equal(t, ProcessingError, err.Type)
		assert.Equal(t, "OCR_FAILED", err.Code)
	})
}

func TestErrorHelpers(t *testing.T) {
	t.Run("is type check", func(t *testing.T) {
		err := NewValidationError("test")
		assert.True(t, IsType(err, ValidationError))
		assert.False(t, IsType(err, ProcessingError))
	})

	t.Run("is code check", func(t *testing.T) {
		err := NewValidationError("test")
		assert.True(t, IsCode(err, "VALIDATION_FAILED"))
		assert.False(t, IsCode(err, "OTHER_CODE"))
	})

	t.Run("get HTTP status", func(t *testing.T) {
		err := NewValidationError("test")
		assert.Equal(t, 400, GetHTTPStatus(err))

		// Non-AppError should return 500
		regularErr := fmt.Errorf("regular error")
		assert.Equal(t, 500, GetHTTPStatus(regularErr))
	})
}

func TestErrorChain(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		chain := NewErrorChain()
		assert.False(t, chain.HasErrors())
		assert.Empty(t, chain.Errors())
		assert.Nil(t, chain.First())
		assert.Empty(t, chain.Error())
	})

	t.Run("single error", func(t *testing.T) {
		chain := NewErrorChain()
		err := NewValidationError("test error")
		chain.Add(err)

		assert.True(t, chain.HasErrors())
		assert.Len(t, chain.Errors(), 1)
		assert.Equal(t, err, chain.First())
		assert.Equal(t, err.Error(), chain.Error())
	})

	t.Run("multiple errors", func(t *testing.T) {
		chain := NewErrorChain()
		err1 := NewValidationError("error 1")
		err2 := NewProcessingError("error 2")

		chain.Add(err1).Add(err2)

		assert.True(t, chain.HasErrors())
		assert.Len(t, chain.Errors(), 2)
		assert.Equal(t, err1, chain.First())
		assert.Contains(t, chain.Error(), "Multiple errors")
	})
}

func TestErrorResponse(t *testing.T) {
	t.Run("create error response", func(t *testing.T) {
		err := NewValidationError("test error")
		response := NewErrorResponse(err)

		assert.Equal(t, err, response.Error)
		assert.False(t, response.Success)
	})
}
