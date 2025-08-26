package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the documents worker
type Config struct {
	Server     ServerConfig
	Redis      RedisConfig
	Worker     WorkerConfig
	External   ExternalConfig
	OCR        OCRConfig
	Cache      CacheConfig
	Logging    LoggingConfig    // v2.0: Structured logging configuration
	Metrics    MetricsConfig    // v2.0: Prometheus metrics configuration
	Validation ValidationConfig // v2.0: Input validation configuration
	Security   SecurityConfig   // v2.0: Security configuration
	Health     HealthConfig     // v2.0: Health check configuration
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Environment  string
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// WorkerConfig holds worker pool configuration
type WorkerConfig struct {
	MaxConcurrency     int
	QueueName          string
	RetryCount         int
	RetryDelay         time.Duration
	MinWorkers         int
	ScaleUpThreshold   int64
	ScaleDownThreshold int64
	CheckInterval      time.Duration
	ScaleDelay         time.Duration
}

// ExternalConfig holds external tools configuration
type ExternalConfig struct {
	VipsEnabled       bool
	FFmpegPath        string
	LibreOfficePath   string
	MutoolPath        string
	TesseractPath     string
	PyMuPDFScript     string
	WkHtmlToPdfPath   string
	PandocPath        string
	NodeJSPath        string // Path to Node.js for Playwright
	PlaywrightEnabled bool   // Enable Playwright PDF generation
}

// OCRConfig holds OCR processing configuration
type OCRConfig struct {
	Language string
	DPI      int
	PSM      int
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled    bool
	TTL        time.Duration
	MaxSize    int64
	Directory  string
	CleanupAge time.Duration
}

// LoggingConfig holds logging configuration for v2.0
type LoggingConfig struct {
	Level      string `json:"level" validate:"oneof=trace debug info warn error fatal panic"`
	Format     string `json:"format" validate:"oneof=json console"`
	Output     string `json:"output" validate:"oneof=stdout stderr file"`
	Filename   string `json:"filename,omitempty"`
	TimeFormat string `json:"time_format"`
	Structured bool   `json:"structured"`
}

// MetricsConfig holds Prometheus metrics configuration for v2.0
type MetricsConfig struct {
	Enabled   bool   `json:"enabled"`
	Port      string `json:"port"`
	Path      string `json:"path"`
	Namespace string `json:"namespace"`
	Subsystem string `json:"subsystem"`
}

// ValidationConfig holds input validation configuration for v2.0
type ValidationConfig struct {
	MaxFileSize        int64    `json:"max_file_size"`
	MinFileSize        int64    `json:"min_file_size"`
	AllowedMimeTypes   []string `json:"allowed_mime_types"`
	AllowedExtensions  []string `json:"allowed_extensions"`
	MaxConcurrentReqs  int      `json:"max_concurrent_reqs"`
	MaxProcessingTime  int      `json:"max_processing_time"`
	RequireContentType bool     `json:"require_content_type"`
	ScanForMalware     bool     `json:"scan_for_malware"`
	MaxChunkSize       int      `json:"max_chunk_size"`
	MinChunkSize       int      `json:"min_chunk_size"`
	MaxChunkOverlap    int      `json:"max_chunk_overlap"`
}

// SecurityConfig holds security configuration for v2.0
type SecurityConfig struct {
	RateLimitEnabled    bool          `json:"rate_limit_enabled"`
	RateLimitPerMinute  int           `json:"rate_limit_per_minute"`
	CorsEnabled         bool          `json:"cors_enabled"`
	CorsAllowedOrigins  []string      `json:"cors_allowed_origins"`
	RequestTimeoutLimit time.Duration `json:"request_timeout_limit"`
	MaxRequestBodySize  int64         `json:"max_request_body_size"`
	TrustedProxies      []string      `json:"trusted_proxies"`
}

// HealthConfig holds health check configuration for v2.0
type HealthConfig struct {
	Enabled       bool          `json:"enabled"`
	Port          string        `json:"port"`
	Path          string        `json:"path"`
	CheckInterval time.Duration `json:"check_interval"`
	Timeout       time.Duration `json:"timeout"`
	ReadinessPath string        `json:"readiness_path"`
	LivenessPath  string        `json:"liveness_path"`
	StartupPath   string        `json:"startup_path"`
}

// Load reads configuration from environment variables and returns Config
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "3001"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 120*time.Second),
			Environment:  getEnv("ENVIRONMENT", "development"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getIntEnv("REDIS_DB", 0),
		},
		Worker: WorkerConfig{
			MaxConcurrency:     getIntEnv("WORKER_MAX_CONCURRENCY", 10),
			QueueName:          getEnv("WORKER_QUEUE_NAME", "documents_queue"),
			RetryCount:         getIntEnv("WORKER_RETRY_COUNT", 3),
			RetryDelay:         getDurationEnv("WORKER_RETRY_DELAY", 5*time.Second),
			MinWorkers:         getIntEnv("WORKER_MIN_WORKERS", 1),
			ScaleUpThreshold:   int64(getIntEnv("WORKER_SCALE_UP_THRESHOLD", 10)),
			ScaleDownThreshold: int64(getIntEnv("WORKER_SCALE_DOWN_THRESHOLD", 2)),
			CheckInterval:      getDurationEnv("WORKER_CHECK_INTERVAL", 10*time.Second),
			ScaleDelay:         getDurationEnv("WORKER_SCALE_DELAY", 30*time.Second),
		},
		External: ExternalConfig{
			VipsEnabled:       getBoolEnv("VIPS_ENABLED", true),
			FFmpegPath:        getEnv("FFMPEG_PATH", "ffmpeg"),
			LibreOfficePath:   getEnv("LIBREOFFICE_PATH", "soffice"),
			MutoolPath:        getEnv("MUTOOL_PATH", "mutool"),
			TesseractPath:     getEnv("TESSERACT_PATH", "tesseract"),
			PyMuPDFScript:     getEnv("PYMUPDF_SCRIPT", "./scripts"),
			WkHtmlToPdfPath:   getEnv("WKHTMLTOPDF_PATH", "wkhtmltopdf"),
			PandocPath:        getEnv("PANDOC_PATH", "pandoc"),
			NodeJSPath:        getEnv("NODEJS_PATH", "node"),
			PlaywrightEnabled: getBoolEnv("PLAYWRIGHT_ENABLED", true),
		},
		OCR: OCRConfig{
			Language: getEnv("OCR_LANGUAGE", "tur+eng"),
			DPI:      getIntEnv("OCR_DPI", 300),
			PSM:      getIntEnv("OCR_PSM", 1),
		},
		Cache: CacheConfig{
			Enabled:    getBoolEnv("CACHE_ENABLED", true),
			TTL:        getDurationEnv("CACHE_TTL", 24*time.Hour),
			MaxSize:    getInt64Env("CACHE_MAX_SIZE", 1024*1024*1024), // 1GB
			Directory:  getEnv("CACHE_DIRECTORY", "./cache"),
			CleanupAge: getDurationEnv("CACHE_CLEANUP_AGE", 7*24*time.Hour), // 7 days
		},
		// v2.0: New configuration sections
		Logging: LoggingConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "json"),
			Output:     getEnv("LOG_OUTPUT", "stdout"),
			Filename:   getEnv("LOG_FILENAME", "logs/app.log"),
			TimeFormat: getEnv("LOG_TIME_FORMAT", "2006-01-02T15:04:05Z07:00"),
			Structured: getBoolEnv("LOG_STRUCTURED", true),
		},
		Metrics: MetricsConfig{
			Enabled:   getBoolEnv("METRICS_ENABLED", true),
			Port:      getEnv("METRICS_PORT", "9090"),
			Path:      getEnv("METRICS_PATH", "/metrics"),
			Namespace: getEnv("METRICS_NAMESPACE", "documents"),
			Subsystem: getEnv("METRICS_SUBSYSTEM", "worker"),
		},
		Validation: ValidationConfig{
			MaxFileSize:        getInt64Env("VALIDATION_MAX_FILE_SIZE", 100*1024*1024), // 100MB
			MinFileSize:        getInt64Env("VALIDATION_MIN_FILE_SIZE", 1),
			MaxConcurrentReqs:  getIntEnv("VALIDATION_MAX_CONCURRENT_REQS", 10),
			MaxProcessingTime:  getIntEnv("VALIDATION_MAX_PROCESSING_TIME", 300), // 5 minutes
			RequireContentType: getBoolEnv("VALIDATION_REQUIRE_CONTENT_TYPE", true),
			ScanForMalware:     getBoolEnv("VALIDATION_SCAN_FOR_MALWARE", false),
			MaxChunkSize:       getIntEnv("VALIDATION_MAX_CHUNK_SIZE", 8000),
			MinChunkSize:       getIntEnv("VALIDATION_MIN_CHUNK_SIZE", 100),
			MaxChunkOverlap:    getIntEnv("VALIDATION_MAX_CHUNK_OVERLAP", 200),
			AllowedMimeTypes: getStringSliceEnv("VALIDATION_ALLOWED_MIME_TYPES", []string{
				"application/pdf",
				"application/msword",
				"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				"application/vnd.ms-excel",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
				"application/vnd.ms-powerpoint",
				"application/vnd.openxmlformats-officedocument.presentationml.presentation",
				"text/plain", "text/markdown", "text/html", "text/csv",
				"image/jpeg", "image/png", "image/webp", "image/avif",
				"video/mp4", "video/avi", "video/quicktime",
			}),
			AllowedExtensions: getStringSliceEnv("VALIDATION_ALLOWED_EXTENSIONS", []string{
				".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
				".txt", ".md", ".html", ".htm", ".csv",
				".jpg", ".jpeg", ".png", ".webp", ".avif",
				".mp4", ".avi", ".mov",
			}),
		},
		Security: SecurityConfig{
			RateLimitEnabled:    getBoolEnv("SECURITY_RATE_LIMIT_ENABLED", true),
			RateLimitPerMinute:  getIntEnv("SECURITY_RATE_LIMIT_PER_MINUTE", 60),
			CorsEnabled:         getBoolEnv("SECURITY_CORS_ENABLED", true),
			CorsAllowedOrigins:  getStringSliceEnv("SECURITY_CORS_ALLOWED_ORIGINS", []string{"*"}),
			RequestTimeoutLimit: getDurationEnv("SECURITY_REQUEST_TIMEOUT_LIMIT", 300*time.Second),
			MaxRequestBodySize:  getInt64Env("SECURITY_MAX_REQUEST_BODY_SIZE", 100*1024*1024), // 100MB
			TrustedProxies:      getStringSliceEnv("SECURITY_TRUSTED_PROXIES", []string{"127.0.0.1", "::1"}),
		},
		Health: HealthConfig{
			Enabled:       getBoolEnv("HEALTH_ENABLED", true),
			Port:          getEnv("HEALTH_PORT", "3002"),
			Path:          getEnv("HEALTH_PATH", "/health"),
			CheckInterval: getDurationEnv("HEALTH_CHECK_INTERVAL", 30*time.Second),
			Timeout:       getDurationEnv("HEALTH_TIMEOUT", 5*time.Second),
			ReadinessPath: getEnv("HEALTH_READINESS_PATH", "/ready"),
			LivenessPath:  getEnv("HEALTH_LIVENESS_PATH", "/live"),
			StartupPath:   getEnv("HEALTH_STARTUP_PATH", "/startup"),
		},
	}
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		log.Printf("Warning: Invalid integer value for %s: %s, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if int64Value, err := strconv.ParseInt(value, 10, 64); err == nil {
			return int64Value
		}
		log.Printf("Warning: Invalid int64 value for %s: %s, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
		log.Printf("Warning: Invalid boolean value for %s: %s, using default: %t", key, value, defaultValue)
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
		log.Printf("Warning: Invalid duration value for %s: %s, using default: %s", key, value, defaultValue)
	}
	return defaultValue
}

func getStringSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim spaces
		var result []string
		for _, item := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// GetDatabaseURL returns the Redis connection URL
func (c *Config) GetRedisURL() string {
	return c.Redis.Host + ":" + c.Redis.Port
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// GetCacheDirectory returns the cache directory path
func (c *Config) GetCacheDirectory() string {
	return c.Cache.Directory
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Check if required external tools are available
	if c.External.VipsEnabled {
		// VIPS is optional, so we don't fail if not found
	}

	// Cache directory validation
	if c.Cache.Enabled {
		if err := os.MkdirAll(c.Cache.Directory, 0755); err != nil {
			log.Printf("Warning: Failed to create cache directory %s: %v", c.Cache.Directory, err)
		}
	}

	return nil
}
