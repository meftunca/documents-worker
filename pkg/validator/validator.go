package validator

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator wraps go-playground/validator with custom validation rules
type Validator struct {
	validate *validator.Validate
}

// Config holds validation configuration
type Config struct {
	MaxFileSize        int64    `json:"max_file_size"`        // Maximum file size in bytes
	AllowedMimeTypes   []string `json:"allowed_mime_types"`   // Allowed MIME types
	AllowedExtensions  []string `json:"allowed_extensions"`   // Allowed file extensions
	MaxConcurrentReqs  int      `json:"max_concurrent_reqs"`  // Maximum concurrent requests
	MaxProcessingTime  int      `json:"max_processing_time"`  // Maximum processing time in seconds
	RequireContentType bool     `json:"require_content_type"` // Require content type header
	ScanForMalware     bool     `json:"scan_for_malware"`     // Enable malware scanning
	MinFileSize        int64    `json:"min_file_size"`        // Minimum file size in bytes
	MaxChunkSize       int      `json:"max_chunk_size"`       // Maximum chunk size for chunking
	MinChunkSize       int      `json:"min_chunk_size"`       // Minimum chunk size for chunking
	MaxChunkOverlap    int      `json:"max_chunk_overlap"`    // Maximum chunk overlap
}

// DefaultConfig returns default validation configuration
func DefaultConfig() *Config {
	return &Config{
		MaxFileSize:        100 * 1024 * 1024, // 100MB
		MinFileSize:        1,                 // 1 byte
		MaxConcurrentReqs:  10,
		MaxProcessingTime:  300, // 5 minutes
		RequireContentType: true,
		ScanForMalware:     false,
		MaxChunkSize:       8000, // 8KB
		MinChunkSize:       100,  // 100 bytes
		MaxChunkOverlap:    200,  // 200 characters
		AllowedMimeTypes: []string{
			"application/pdf",
			"application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.ms-excel",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.ms-powerpoint",
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
			"text/plain",
			"text/markdown",
			"text/html",
			"text/csv",
			"image/jpeg",
			"image/png",
			"image/webp",
			"image/avif",
			"video/mp4",
			"video/avi",
			"video/quicktime",
		},
		AllowedExtensions: []string{
			".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			".txt", ".md", ".html", ".htm", ".csv",
			".jpg", ".jpeg", ".png", ".webp", ".avif",
			".mp4", ".avi", ".mov",
		},
	}
}

// New creates a new validator instance
func New(config *Config) *Validator {
	if config == nil {
		config = DefaultConfig()
	}

	validate := validator.New()

	// Register custom validation tags
	validate.RegisterValidation("file_size", validateFileSize(config.MinFileSize, config.MaxFileSize))
	validate.RegisterValidation("mime_type", validateMimeType(config.AllowedMimeTypes))
	validate.RegisterValidation("file_extension", validateFileExtension(config.AllowedExtensions))
	validate.RegisterValidation("chunk_size", validateChunkSize(config.MinChunkSize, config.MaxChunkSize))
	validate.RegisterValidation("chunk_overlap", validateChunkOverlap(config.MaxChunkOverlap))

	return &Validator{
		validate: validate,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	var messages []string
	for _, err := range v {
		messages = append(messages, err.Message)
	}
	return strings.Join(messages, "; ")
}

// ValidateStruct validates a struct
func (v *Validator) ValidateStruct(s interface{}) error {
	err := v.validate.Struct(s)
	if err != nil {
		var validationErrors ValidationErrors
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors = append(validationErrors, ValidationError{
				Field:   err.Field(),
				Tag:     err.Tag(),
				Value:   fmt.Sprintf("%v", err.Value()),
				Message: getErrorMessage(err),
			})
		}
		return validationErrors
	}
	return nil
}

// ValidateFile validates an uploaded file
func (v *Validator) ValidateFile(file *multipart.FileHeader, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	var errors ValidationErrors

	// Validate file size
	if file.Size > config.MaxFileSize {
		errors = append(errors, ValidationError{
			Field:   "file_size",
			Tag:     "max_size",
			Value:   fmt.Sprintf("%d", file.Size),
			Message: fmt.Sprintf("File size %d bytes exceeds maximum allowed size of %d bytes", file.Size, config.MaxFileSize),
		})
	}

	if file.Size < config.MinFileSize {
		errors = append(errors, ValidationError{
			Field:   "file_size",
			Tag:     "min_size",
			Value:   fmt.Sprintf("%d", file.Size),
			Message: fmt.Sprintf("File size %d bytes is below minimum required size of %d bytes", file.Size, config.MinFileSize),
		})
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !contains(config.AllowedExtensions, ext) {
		errors = append(errors, ValidationError{
			Field:   "file_extension",
			Tag:     "allowed_extension",
			Value:   ext,
			Message: fmt.Sprintf("File extension '%s' is not allowed. Allowed extensions: %v", ext, config.AllowedExtensions),
		})
	}

	// Validate MIME type if header is available
	if config.RequireContentType && file.Header != nil {
		contentType := file.Header.Get("Content-Type")
		if contentType == "" {
			errors = append(errors, ValidationError{
				Field:   "content_type",
				Tag:     "required",
				Value:   "",
				Message: "Content-Type header is required",
			})
		} else if !contains(config.AllowedMimeTypes, contentType) {
			errors = append(errors, ValidationError{
				Field:   "content_type",
				Tag:     "allowed_mime_type",
				Value:   contentType,
				Message: fmt.Sprintf("MIME type '%s' is not allowed. Allowed types: %v", contentType, config.AllowedMimeTypes),
			})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// ValidateChunkingRequest validates chunking request parameters
func (v *Validator) ValidateChunkingRequest(chunkSize, overlap int, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	var errors ValidationErrors

	if chunkSize < config.MinChunkSize {
		errors = append(errors, ValidationError{
			Field:   "chunk_size",
			Tag:     "min_chunk_size",
			Value:   fmt.Sprintf("%d", chunkSize),
			Message: fmt.Sprintf("Chunk size %d is below minimum of %d", chunkSize, config.MinChunkSize),
		})
	}

	if chunkSize > config.MaxChunkSize {
		errors = append(errors, ValidationError{
			Field:   "chunk_size",
			Tag:     "max_chunk_size",
			Value:   fmt.Sprintf("%d", chunkSize),
			Message: fmt.Sprintf("Chunk size %d exceeds maximum of %d", chunkSize, config.MaxChunkSize),
		})
	}

	if overlap > config.MaxChunkOverlap {
		errors = append(errors, ValidationError{
			Field:   "chunk_overlap",
			Tag:     "max_chunk_overlap",
			Value:   fmt.Sprintf("%d", overlap),
			Message: fmt.Sprintf("Chunk overlap %d exceeds maximum of %d", overlap, config.MaxChunkOverlap),
		})
	}

	if overlap >= chunkSize {
		errors = append(errors, ValidationError{
			Field:   "chunk_overlap",
			Tag:     "overlap_less_than_size",
			Value:   fmt.Sprintf("%d", overlap),
			Message: fmt.Sprintf("Chunk overlap %d must be less than chunk size %d", overlap, chunkSize),
		})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// IsSuspiciousFile checks for potentially malicious files
func (v *Validator) IsSuspiciousFile(filename string, content []byte) (bool, string) {
	// Check for suspicious file names
	suspiciousPatterns := []string{
		"../", "..\\", // Path traversal
		"<script", "javascript:", // Script injection
		"<?php", "<%", // Server-side scripts
		"cmd.exe", "powershell", // Executables
	}

	filename = strings.ToLower(filename)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(filename, pattern) {
			return true, fmt.Sprintf("Suspicious filename pattern detected: %s", pattern)
		}
	}

	// Check file content for suspicious patterns (first 1KB)
	if len(content) > 0 {
		contentStr := strings.ToLower(string(content[:min(len(content), 1024)]))
		for _, pattern := range suspiciousPatterns {
			if strings.Contains(contentStr, pattern) {
				return true, fmt.Sprintf("Suspicious content pattern detected: %s", pattern)
			}
		}
	}

	return false, ""
}

// Custom validation functions
func validateFileSize(minSize, maxSize int64) validator.Func {
	return func(fl validator.FieldLevel) bool {
		size := fl.Field().Int()
		return size >= minSize && size <= maxSize
	}
}

func validateMimeType(allowedTypes []string) validator.Func {
	return func(fl validator.FieldLevel) bool {
		mimeType := fl.Field().String()
		return contains(allowedTypes, mimeType)
	}
}

func validateFileExtension(allowedExtensions []string) validator.Func {
	return func(fl validator.FieldLevel) bool {
		ext := strings.ToLower(fl.Field().String())
		return contains(allowedExtensions, ext)
	}
}

func validateChunkSize(minSize, maxSize int) validator.Func {
	return func(fl validator.FieldLevel) bool {
		size := int(fl.Field().Int())
		return size >= minSize && size <= maxSize
	}
}

func validateChunkOverlap(maxOverlap int) validator.Func {
	return func(fl validator.FieldLevel) bool {
		overlap := int(fl.Field().Int())
		return overlap <= maxOverlap
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getErrorMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", err.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s", err.Field(), err.Param())
	case "max":
		return fmt.Sprintf("%s must not exceed %s", err.Field(), err.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", err.Field())
	case "url":
		return fmt.Sprintf("%s must be a valid URL", err.Field())
	case "file_size":
		return fmt.Sprintf("%s has invalid file size", err.Field())
	case "mime_type":
		return fmt.Sprintf("%s has unsupported MIME type", err.Field())
	case "file_extension":
		return fmt.Sprintf("%s has unsupported file extension", err.Field())
	case "chunk_size":
		return fmt.Sprintf("%s has invalid chunk size", err.Field())
	case "chunk_overlap":
		return fmt.Sprintf("%s has invalid chunk overlap", err.Field())
	default:
		return fmt.Sprintf("%s is invalid", err.Field())
	}
}

// Global validator instance
var globalValidator *Validator

// Init initializes the global validator
func Init(config *Config) {
	globalValidator = New(config)
}

// Get returns the global validator
func Get() *Validator {
	if globalValidator == nil {
		globalValidator = New(DefaultConfig())
	}
	return globalValidator
}
