package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Manager handles advanced configuration management
type Manager struct {
	config       *Config
	watchers     []ConfigWatcher
	mu           sync.RWMutex
	configPaths  []string
	fileWatcher  *fsnotify.Watcher
	reloadChan   chan bool
	stopChan     chan bool
	environment  string
	featureFlags map[string]bool
}

// ConfigWatcher is called when configuration changes
type ConfigWatcher func(oldConfig, newConfig *Config) error

// ValidationRule defines a configuration validation rule
type ValidationRule struct {
	Field       string
	Required    bool
	MinValue    interface{}
	MaxValue    interface{}
	AllowedVals []interface{}
	CustomFn    func(value interface{}) error
}

// EnvironmentConfig holds environment-specific settings
type EnvironmentConfig struct {
	Development *Config `json:"development,omitempty"`
	Staging     *Config `json:"staging,omitempty"`
	Production  *Config `json:"production,omitempty"`
}

// FeatureFlag represents a feature toggle
type FeatureFlag struct {
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	Description string                 `json:"description"`
	Conditions  map[string]interface{} `json:"conditions,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// NewManager creates a new configuration manager
func NewManager(environment string) *Manager {
	return &Manager{
		environment:  environment,
		watchers:     make([]ConfigWatcher, 0),
		configPaths:  make([]string, 0),
		reloadChan:   make(chan bool, 1),
		stopChan:     make(chan bool, 1),
		featureFlags: make(map[string]bool),
	}
}

// LoadFromFile loads configuration from a file
func (m *Manager) LoadFromFile(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	// Parse environment-specific config
	var envConfig EnvironmentConfig
	if err := json.Unmarshal(data, &envConfig); err != nil {
		// Fallback to direct config parsing
		var config Config
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config file %s: %w", filePath, err)
		}
		m.config = &config
	} else {
		// Select environment-specific config
		switch m.environment {
		case "development":
			if envConfig.Development != nil {
				m.config = envConfig.Development
			}
		case "staging":
			if envConfig.Staging != nil {
				m.config = envConfig.Staging
			}
		case "production":
			if envConfig.Production != nil {
				m.config = envConfig.Production
			}
		default:
			// For test or other environments, try to use any available config
			if envConfig.Development != nil {
				m.config = envConfig.Development
			} else if envConfig.Production != nil {
				m.config = envConfig.Production
			} else if envConfig.Staging != nil {
				m.config = envConfig.Staging
			} else {
				return fmt.Errorf("no configuration found for environment: %s", m.environment)
			}
		}

		if m.config == nil {
			return fmt.Errorf("no configuration found for environment: %s", m.environment)
		}
	}

	// Override with environment variables
	if err := m.overrideWithEnvVars(); err != nil {
		return fmt.Errorf("failed to override with env vars: %w", err)
	}

	// Add to watched paths
	m.configPaths = append(m.configPaths, filePath)

	return nil
}

// LoadFromEnv loads configuration from environment variables only
func (m *Manager) LoadFromEnv() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Start with default config
	m.config = &Config{
		Server: ServerConfig{
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
			Environment:  "development",
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			TimeFormat: time.RFC3339,
		},
		Metrics: MetricsConfig{
			Enabled:   true,
			Port:      "9090",
			Path:      "/metrics",
			Namespace: "documents",
			Subsystem: "worker",
		},
		Validation: ValidationConfig{
			MaxFileSize:        100 * 1024 * 1024, // 100MB
			MinFileSize:        1,
			MaxConcurrentReqs:  100,
			RequireContentType: true,
			ScanForMalware:     true,
		},
		Security: SecurityConfig{
			RateLimitEnabled:   true,
			RateLimitPerMinute: 60,
			CorsEnabled:        true,
		},
		Health: HealthConfig{
			Enabled: true,
			Path:    "/health",
		},
	}

	// Override with environment variables
	return m.overrideWithEnvVars()
}

// StartWatching starts watching configuration files for changes
func (m *Manager) StartWatching() error {
	if len(m.configPaths) == 0 {
		return nil // No files to watch
	}

	var err error
	m.fileWatcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Add all config files to watcher
	for _, path := range m.configPaths {
		if err := m.fileWatcher.Add(path); err != nil {
			return fmt.Errorf("failed to add file to watcher %s: %w", path, err)
		}
	}

	// Start the watching goroutine
	go m.watchLoop()

	return nil
}

// StopWatching stops watching configuration files
func (m *Manager) StopWatching() {
	if m.fileWatcher != nil {
		m.stopChan <- true
		m.fileWatcher.Close()
	}
}

// AddWatcher adds a configuration change watcher
func (m *Manager) AddWatcher(watcher ConfigWatcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchers = append(m.watchers, watcher)
}

// GetConfig returns the current configuration (thread-safe)
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	configCopy := *m.config
	return &configCopy
}

// Reload forces a configuration reload
func (m *Manager) Reload() error {
	oldConfig := m.GetConfig()

	if len(m.configPaths) == 0 {
		// No config files, reload from env
		if err := m.LoadFromEnv(); err != nil {
			return err
		}
	} else {
		// Reload from the first config file
		if err := m.LoadFromFile(m.configPaths[0]); err != nil {
			return err
		}
	}

	newConfig := m.GetConfig()

	// Notify watchers
	return m.notifyWatchers(oldConfig, newConfig)
} // Validate validates the current configuration
func (m *Manager) Validate(rules []ValidationRule) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := m.config
	configValue := reflect.ValueOf(config).Elem()

	for _, rule := range rules {
		field := configValue.FieldByName(rule.Field)
		if !field.IsValid() {
			return fmt.Errorf("field %s not found", rule.Field)
		}

		// Check if required field is zero value
		if rule.Required && field.IsZero() {
			return fmt.Errorf("required field %s is not set", rule.Field)
		}

		// Skip validation for zero values of non-required fields
		if !rule.Required && field.IsZero() {
			continue
		}

		// Validate based on field type
		if err := m.validateField(rule, field); err != nil {
			return fmt.Errorf("validation failed for field %s: %w", rule.Field, err)
		}
	}

	return nil
}

// SetFeatureFlag sets a feature flag
func (m *Manager) SetFeatureFlag(name string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.featureFlags[name] = enabled
}

// IsFeatureEnabled checks if a feature flag is enabled
func (m *Manager) IsFeatureEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	enabled, exists := m.featureFlags[name]
	return exists && enabled
}

// GetFeatureFlags returns all feature flags
func (m *Manager) GetFeatureFlags() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flags := make(map[string]bool)
	for k, v := range m.featureFlags {
		flags[k] = v
	}
	return flags
}

// watchLoop watches for file changes
func (m *Manager) watchLoop() {
	for {
		select {
		case event, ok := <-m.fileWatcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				// Debounce rapid writes
				select {
				case m.reloadChan <- true:
				default:
				}
				// Wait a bit for file to be fully written
				time.Sleep(100 * time.Millisecond)
				select {
				case <-m.reloadChan:
					if err := m.Reload(); err != nil {
						// Log error but continue watching
						fmt.Printf("Failed to reload config: %v\n", err)
					}
				default:
				}
			}
		case err, ok := <-m.fileWatcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Config watcher error: %v\n", err)
		case <-m.stopChan:
			return
		}
	}
}

// overrideWithEnvVars overrides configuration with environment variables
func (m *Manager) overrideWithEnvVars() error {
	return m.overrideStruct(reflect.ValueOf(m.config).Elem(), "")
}

// overrideStruct recursively overrides struct fields with environment variables
func (m *Manager) overrideStruct(value reflect.Value, prefix string) error {
	valueType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := valueType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Build environment variable name
		envName := prefix + strings.ToUpper(fieldType.Name)
		envName = strings.ReplaceAll(envName, "CONFIG", "")

		if field.Kind() == reflect.Struct {
			// Recursively handle nested structs
			if err := m.overrideStruct(field, envName+"_"); err != nil {
				return err
			}
			continue
		}

		// Get environment variable value
		envValue := os.Getenv(envName)
		if envValue == "" {
			continue
		}

		// Set the field value based on its type
		if err := m.setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("failed to set field %s from env var %s: %w", fieldType.Name, envName, err)
		}
	}

	return nil
}

// setFieldValue sets a field value from a string
func (m *Manager) setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(intVal)
		}
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)
	case reflect.Slice:
		// Handle string slices (comma-separated values)
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			for i, v := range values {
				values[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(values))
		}
	}

	return nil
}

// validateField validates a single field
func (m *Manager) validateField(rule ValidationRule, field reflect.Value) error {
	// Custom validation function takes precedence
	if rule.CustomFn != nil {
		return rule.CustomFn(field.Interface())
	}

	// Check allowed values
	if len(rule.AllowedVals) > 0 {
		found := false
		for _, allowed := range rule.AllowedVals {
			if reflect.DeepEqual(field.Interface(), allowed) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value not in allowed list")
		}
	}

	// Check min/max values for numeric types
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal := field.Int()
		if rule.MinValue != nil {
			if minVal, ok := rule.MinValue.(int64); ok && intVal < minVal {
				return fmt.Errorf("value %d is less than minimum %d", intVal, minVal)
			}
		}
		if rule.MaxValue != nil {
			if maxVal, ok := rule.MaxValue.(int64); ok && intVal > maxVal {
				return fmt.Errorf("value %d is greater than maximum %d", intVal, maxVal)
			}
		}
	case reflect.Float32, reflect.Float64:
		floatVal := field.Float()
		if rule.MinValue != nil {
			if minVal, ok := rule.MinValue.(float64); ok && floatVal < minVal {
				return fmt.Errorf("value %f is less than minimum %f", floatVal, minVal)
			}
		}
		if rule.MaxValue != nil {
			if maxVal, ok := rule.MaxValue.(float64); ok && floatVal > maxVal {
				return fmt.Errorf("value %f is greater than maximum %f", floatVal, maxVal)
			}
		}
	case reflect.String:
		strVal := field.String()
		if rule.MinValue != nil {
			if minLen, ok := rule.MinValue.(int); ok && len(strVal) < minLen {
				return fmt.Errorf("string length %d is less than minimum %d", len(strVal), minLen)
			}
		}
		if rule.MaxValue != nil {
			if maxLen, ok := rule.MaxValue.(int); ok && len(strVal) > maxLen {
				return fmt.Errorf("string length %d is greater than maximum %d", len(strVal), maxLen)
			}
		}
	}

	return nil
}

// notifyWatchers notifies all configuration watchers
func (m *Manager) notifyWatchers(oldConfig, newConfig *Config) error {
	for _, watcher := range m.watchers {
		if err := watcher(oldConfig, newConfig); err != nil {
			return fmt.Errorf("config watcher failed: %w", err)
		}
	}
	return nil
}

// ExportToFile exports current configuration to a file
func (m *Manager) ExportToFile(filePath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal configuration to JSON
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filePath, err)
	}

	return nil
}
