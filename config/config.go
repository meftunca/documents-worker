package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the documents worker
type Config struct {
	Server   ServerConfig
	Redis    RedisConfig
	Worker   WorkerConfig
	External ExternalConfig
	OCR      OCRConfig
	Cache    CacheConfig
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
