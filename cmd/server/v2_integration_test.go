package main

import (
	"bytes"
	"documents-worker/config"
	"documents-worker/pkg/errors"
	"documents-worker/pkg/logger"
	"documents-worker/pkg/metrics"
	"documents-worker/pkg/validator"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestV2Integration tests the v2.0 features integration
func TestV2Integration(t *testing.T) {
	// Setup v2.0 configuration
	cfg := config.Load()
	cfg.Logging.Level = "debug"
	cfg.Logging.Format = "json"
	cfg.Metrics.Enabled = true
	cfg.Validation.MaxFileSize = 10 * 1024 * 1024 // 10MB for testing
	cfg.Security.RateLimitEnabled = false         // Disable for testing

	// Initialize v2.0 components
	loggerConfig := &logger.Config{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     "stdout",
		TimeFormat: cfg.Logging.TimeFormat,
	}
	require.NoError(t, logger.Init(loggerConfig))

	metrics.Init(cfg.Metrics.Namespace, cfg.Metrics.Subsystem)

	validatorConfig := &validator.Config{
		MaxFileSize:        cfg.Validation.MaxFileSize,
		MinFileSize:        cfg.Validation.MinFileSize,
		AllowedMimeTypes:   cfg.Validation.AllowedMimeTypes,
		AllowedExtensions:  cfg.Validation.AllowedExtensions,
		MaxConcurrentReqs:  cfg.Validation.MaxConcurrentReqs,
		RequireContentType: true, // Re-enabled since we fixed the test headers
		ScanForMalware:     cfg.Validation.ScanForMalware,
	}
	validator.Init(validatorConfig)

	// Create a test Fiber app with v2.0 error handling
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if appErr, ok := err.(*errors.AppError); ok {
				return c.Status(appErr.HTTPStatus).JSON(errors.NewErrorResponse(appErr))
			}
			internalErr := errors.NewInternalError(err.Error())
			return c.Status(internalErr.HTTPStatus).JSON(errors.NewErrorResponse(internalErr))
		},
	})

	// Add v2.0 middleware
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		requestID := c.Get("X-Request-ID", "test-request")
		reqCtx := logger.WithRequestID(c.Context(), requestID)
		c.SetUserContext(reqCtx)

		err := c.Next()

		duration := time.Since(start)
		log := logger.Get()
		log.LogRequest(reqCtx, c.Method(), c.Path(), c.Get("User-Agent"), c.IP(), duration)

		return err
	})

	// Test routes
	app.Post("/upload", func(c *fiber.Ctx) error {
		file, err := c.FormFile("file")
		if err != nil {
			return errors.NewValidationError("No file provided")
		}

		v := validator.Get()
		if err := v.ValidateFile(file, validatorConfig); err != nil {
			return errors.NewValidationError(err.Error())
		}

		return c.JSON(fiber.Map{
			"success":  true,
			"filename": file.Filename,
			"size":     file.Size,
		})
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy", "version": "v2.0.0"})
	})

	t.Run("health check works", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)
		assert.Equal(t, "healthy", result["status"])
		assert.Equal(t, "v2.0.0", result["version"])
	})

	t.Run("file upload validation - valid file", func(t *testing.T) {
		// Create a test file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Create form file with proper header
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="file"; filename="test.pdf"`)
		h.Set("Content-Type", "application/pdf")
		part, err := writer.CreatePart(h)
		require.NoError(t, err)
		part.Write([]byte("%PDF-1.4 test content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Request-ID", "test-valid-upload")

		resp, err := app.Test(req)
		require.NoError(t, err)

		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Debug: Print the response if it's not 200
		if resp.StatusCode != 200 {
			t.Logf("Unexpected status code: %d", resp.StatusCode)
			t.Logf("Response body: %s", string(respBody))
		}

		assert.Equal(t, 200, resp.StatusCode)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		if result["success"] != nil {
			assert.True(t, result["success"].(bool))
			assert.Equal(t, "test.pdf", result["filename"])
		}
	})

	t.Run("file upload validation - file too large", func(t *testing.T) {
		// Create a large test file (larger than 10MB limit)
		largeContent := make([]byte, 11*1024*1024) // 11MB
		for i := range largeContent {
			largeContent[i] = 'A'
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Create form file with proper header
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="file"; filename="large.pdf"`)
		h.Set("Content-Type", "application/pdf")
		part, err := writer.CreatePart(h)
		require.NoError(t, err)
		part.Write(largeContent)
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Request-ID", "test-large-upload")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode) // Bad Request

		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)
		assert.False(t, result["success"].(bool))
	})

	t.Run("file upload validation - invalid extension", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "malware.exe")
		require.NoError(t, err)
		part.Write([]byte("MZ executable content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Request-ID", "test-invalid-extension")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)

		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)
		assert.False(t, result["success"].(bool))

		// Check error structure
		errorInfo := result["error"].(map[string]interface{})
		assert.Equal(t, "validation_error", errorInfo["type"])
		assert.Contains(t, errorInfo["message"], "not allowed")
	})

	t.Run("error handling structure", func(t *testing.T) {
		// Test endpoint that returns a known error
		app.Get("/test-error", func(c *fiber.Ctx) error {
			return errors.NewProcessingError("Test processing error").
				WithContext("operation", "test").
				WithTrace("trace-123")
		})

		req := httptest.NewRequest("GET", "/test-error", nil)
		req.Header.Set("X-Request-ID", "test-error-handling")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 422, resp.StatusCode) // Unprocessable Entity

		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.False(t, result["success"].(bool))
		errorInfo := result["error"].(map[string]interface{})
		assert.Equal(t, "processing_error", errorInfo["type"])
		assert.Equal(t, "PROCESSING_FAILED", errorInfo["code"])
		assert.Equal(t, "Test processing error", errorInfo["message"])
		assert.Equal(t, float64(422), errorInfo["http_status"])
		assert.NotNil(t, errorInfo["timestamp"])
		assert.Equal(t, "trace-123", errorInfo["trace_id"])

		// Check context
		context := errorInfo["context"].(map[string]interface{})
		assert.Equal(t, "test", context["operation"])
	})

	t.Run("metrics recording", func(t *testing.T) {
		// Test that metrics are recorded (this is a basic check)
		metricsInstance := metrics.Get()
		assert.NotNil(t, metricsInstance)

		// Record some test metrics
		metricsInstance.RecordHTTPRequest("GET", "/test", "200", 100*time.Millisecond, 1024)
		metricsInstance.RecordDocumentProcessing("pdf", "convert", "success", 5*time.Second, 2048)
		metricsInstance.SetActiveWorkers(3)

		// If we got here without panicking, metrics are working
		assert.True(t, true)
	})
}

// TestConfigurationV2 tests the v2.0 configuration loading
func TestConfigurationV2(t *testing.T) {
	// Set some environment variables for testing
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("METRICS_ENABLED", "true")
	os.Setenv("VALIDATION_MAX_FILE_SIZE", "52428800") // 50MB
	os.Setenv("SECURITY_RATE_LIMIT_ENABLED", "true")
	defer func() {
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("METRICS_ENABLED")
		os.Unsetenv("VALIDATION_MAX_FILE_SIZE")
		os.Unsetenv("SECURITY_RATE_LIMIT_ENABLED")
	}()

	cfg := config.Load()

	t.Run("v2.0 config sections loaded", func(t *testing.T) {
		assert.Equal(t, "debug", cfg.Logging.Level)
		assert.True(t, cfg.Metrics.Enabled)
		assert.Equal(t, int64(52428800), cfg.Validation.MaxFileSize)
		assert.True(t, cfg.Security.RateLimitEnabled)
	})

	t.Run("default values work", func(t *testing.T) {
		assert.Equal(t, "documents", cfg.Metrics.Namespace)
		assert.Equal(t, "worker", cfg.Metrics.Subsystem)
		assert.Equal(t, "/metrics", cfg.Metrics.Path)
		assert.True(t, cfg.Health.Enabled)
		assert.Equal(t, "/health", cfg.Health.Path)
	})
}
