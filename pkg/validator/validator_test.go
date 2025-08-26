package validator

import (
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatorConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   bool
	}{
		{
			name:   "default config",
			config: DefaultConfig(),
			want:   true,
		},
		{
			name: "custom config",
			config: &Config{
				MaxFileSize:        50 * 1024 * 1024, // 50MB
				MinFileSize:        1,
				MaxConcurrentReqs:  5,
				RequireContentType: true,
				AllowedMimeTypes:   []string{"application/pdf"},
				AllowedExtensions:  []string{".pdf"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := New(tt.config)
			assert.NotNil(t, validator)
		})
	}
}

func TestFileValidation(t *testing.T) {
	validator := New(DefaultConfig())
	config := DefaultConfig()

	tests := []struct {
		name      string
		file      *multipart.FileHeader
		expectErr bool
	}{
		{
			name: "valid PDF file",
			file: &multipart.FileHeader{
				Filename: "test.pdf",
				Size:     1024 * 1024, // 1MB
				Header: textproto.MIMEHeader{
					"Content-Type": []string{"application/pdf"},
				},
			},
			expectErr: false,
		},
		{
			name: "file too large",
			file: &multipart.FileHeader{
				Filename: "large.pdf",
				Size:     200 * 1024 * 1024, // 200MB
				Header: textproto.MIMEHeader{
					"Content-Type": []string{"application/pdf"},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid extension",
			file: &multipart.FileHeader{
				Filename: "test.exe",
				Size:     1024,
				Header: textproto.MIMEHeader{
					"Content-Type": []string{"application/octet-stream"},
				},
			},
			expectErr: true,
		},
		{
			name: "file too small",
			file: &multipart.FileHeader{
				Filename: "empty.pdf",
				Size:     0,
				Header: textproto.MIMEHeader{
					"Content-Type": []string{"application/pdf"},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateFile(tt.file, config)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChunkingValidation(t *testing.T) {
	validator := New(DefaultConfig())
	config := DefaultConfig()

	tests := []struct {
		name      string
		chunkSize int
		overlap   int
		expectErr bool
	}{
		{
			name:      "valid chunking parameters",
			chunkSize: 4000,
			overlap:   100,
			expectErr: false,
		},
		{
			name:      "chunk size too small",
			chunkSize: 50,
			overlap:   10,
			expectErr: true,
		},
		{
			name:      "chunk size too large",
			chunkSize: 10000,
			overlap:   100,
			expectErr: true,
		},
		{
			name:      "overlap too large",
			chunkSize: 2000,
			overlap:   500,
			expectErr: true,
		},
		{
			name:      "overlap >= chunk size",
			chunkSize: 1000,
			overlap:   1000,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateChunkingRequest(tt.chunkSize, tt.overlap, config)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSuspiciousFileDetection(t *testing.T) {
	validator := New(DefaultConfig())

	tests := []struct {
		name      string
		filename  string
		content   []byte
		expectSus bool
	}{
		{
			name:      "normal PDF file",
			filename:  "document.pdf",
			content:   []byte("%PDF-1.4"),
			expectSus: false,
		},
		{
			name:      "path traversal in filename",
			filename:  "../../../etc/passwd",
			content:   []byte("normal content"),
			expectSus: true,
		},
		{
			name:      "script in filename",
			filename:  "test<script>alert(1)</script>.pdf",
			content:   []byte("normal content"),
			expectSus: true,
		},
		{
			name:      "script in content",
			filename:  "normal.txt",
			content:   []byte("Hello <script>alert('xss')</script> world"),
			expectSus: true,
		},
		{
			name:      "executable reference",
			filename:  "test.txt",
			content:   []byte("run cmd.exe /c dir"),
			expectSus: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSuspicious, reason := validator.IsSuspiciousFile(tt.filename, tt.content)
			assert.Equal(t, tt.expectSus, isSuspicious)
			if tt.expectSus {
				assert.NotEmpty(t, reason)
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	t.Run("validation error chain", func(t *testing.T) {
		errors := ValidationErrors{
			{Field: "file_size", Message: "File too large"},
			{Field: "file_type", Message: "Invalid type"},
		}

		errMsg := errors.Error()
		assert.Contains(t, errMsg, "File too large")
		assert.Contains(t, errMsg, "Invalid type")
	})
}
