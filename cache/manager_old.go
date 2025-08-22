package cache

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// CacheManager handles file caching for processed documents
type CacheManager struct {
	cacheDir string
	ttl      time.Duration
	enabled  bool
}

// CacheEntry represents a cached item
type CacheEntry struct {
	Key         string    `json:"key"`
	FilePath    string    `json:"file_path"`
	OutputPath  string    `json:"output_path"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	FileSize    int64     `json:"file_size"`
	ProcessType string    `json:"process_type"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewCacheManager creates a new cache manager
func NewCacheManager(cacheDir string, ttl time.Duration, enabled bool) *CacheManager {
	if enabled {
		os.MkdirAll(cacheDir, 0755)
	}
	
	return &CacheManager{
		cacheDir: cacheDir,
		ttl:      ttl,
		enabled:  enabled,
	}
}

// GetCacheKey generates a cache key based on file content and processing options
func (cm *CacheManager) GetCacheKey(filePath string, processType string, options interface{}) (string, error) {
	if !cm.enabled {
		return "", fmt.Errorf("cache disabled")
	}
	
	// Read file for hash
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	
	// Create hash from file content, size, mod time, and options
	hasher := md5.New()
	
	// Hash file content (first 1MB for performance)
	buffer := make([]byte, 1024*1024)
	n, _ := file.Read(buffer)
	hasher.Write(buffer[:n])
	
	// Hash metadata
	metadata := fmt.Sprintf("%s-%d-%d-%s", 
		filePath, 
		fileInfo.Size(), 
		fileInfo.ModTime().Unix(),
		processType,
	)
	hasher.Write([]byte(metadata))
	
	// Hash options
	if options != nil {
		optionsJSON, _ := json.Marshal(options)
		hasher.Write(optionsJSON)
	}
	
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Get retrieves a cached result if it exists and is valid
func (cm *CacheManager) Get(cacheKey string) (*CacheEntry, error) {
	if !cm.enabled {
		return nil, fmt.Errorf("cache disabled")
	}
	
	entryPath := filepath.Join(cm.cacheDir, cacheKey+".json")
	
	// Check if entry exists
	if _, err := os.Stat(entryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache miss")
	}
	
	// Read entry
	data, err := os.ReadFile(entryPath)
	if err != nil {
		return nil, err
	}
	
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	
	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		cm.Delete(cacheKey)
		return nil, fmt.Errorf("cache expired")
	}
	
	// Check if output file still exists
	if _, err := os.Stat(entry.OutputPath); os.IsNotExist(err) {
		cm.Delete(cacheKey)
		return nil, fmt.Errorf("cached file missing")
	}
	
	return &entry, nil
}

// Set stores a result in cache
func (cm *CacheManager) Set(cacheKey, inputPath, outputPath, processType string, metadata map[string]interface{}) error {
	if !cm.enabled {
		return nil // No error, just skip
	}
	
	// Get output file info
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		return err
	}
	
	// Create cache entry
	entry := CacheEntry{
		Key:         cacheKey,
		FilePath:    inputPath,
		OutputPath:  outputPath,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(cm.ttl),
		FileSize:    outputInfo.Size(),
		ProcessType: processType,
		Metadata:    metadata,
	}
	
	// Save entry
	entryPath := filepath.Join(cm.cacheDir, cacheKey+".json")
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(entryPath, data, 0644)
}

// Delete removes a cache entry
func (cm *CacheManager) Delete(cacheKey string) error {
	if !cm.enabled {
		return nil
	}
	
	entryPath := filepath.Join(cm.cacheDir, cacheKey+".json")
	
	// Read entry to get output path
	if data, err := os.ReadFile(entryPath); err == nil {
		var entry CacheEntry
		if json.Unmarshal(data, &entry) == nil {
			// Delete output file
			os.Remove(entry.OutputPath)
		}
	}
	
	// Delete entry file
	return os.Remove(entryPath)
}

// CleanExpired removes expired cache entries
func (cm *CacheManager) CleanExpired() error {
	if !cm.enabled {
		return nil
	}
	
	entries, err := filepath.Glob(filepath.Join(cm.cacheDir, "*.json"))
	if err != nil {
		return err
	}
	
	now := time.Now()
	cleaned := 0
	
	for _, entryPath := range entries {
		data, err := os.ReadFile(entryPath)
		if err != nil {
			continue
		}
		
		var entry CacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		
		if now.After(entry.ExpiresAt) {
			if err := cm.Delete(entry.Key); err == nil {
				cleaned++
			}
		}
	}
	
	if cleaned > 0 {
		fmt.Printf("Cleaned %d expired cache entries\n", cleaned)
	}
	
	return nil
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled":    cm.enabled,
		"cache_dir":  cm.cacheDir,
		"ttl_hours":  cm.ttl.Hours(),
		"total_entries": 0,
		"total_size":    int64(0),
		"expired":       0,
	}
	
	if !cm.enabled {
		return stats
	}
	
	entries, err := filepath.Glob(filepath.Join(cm.cacheDir, "*.json"))
	if err != nil {
		return stats
	}
	
	now := time.Now()
	expired := 0
	totalSize := int64(0)
	
	for _, entryPath := range entries {
		data, err := os.ReadFile(entryPath)
		if err != nil {
			continue
		}
		
		var entry CacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		
		totalSize += entry.FileSize
		
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}
	
	stats["total_entries"] = len(entries)
	stats["total_size"] = totalSize
	stats["expired"] = expired
	
	return stats
}

// WarmupCache pre-processes common file types for better performance
func (cm *CacheManager) WarmupCache(commonFiles []string, processor func(string) error) error {
	if !cm.enabled {
		return nil
	}
	
	for _, filePath := range commonFiles {
		if _, err := os.Stat(filePath); err == nil {
			// Check if already cached
			cacheKey, err := cm.GetCacheKey(filePath, "warmup", nil)
			if err != nil {
				continue
			}
			
			if _, err := cm.Get(cacheKey); err != nil {
				// Not cached, process it
				if processor != nil {
					go processor(filePath) // Process in background
				}
			}
		}
	}
	
	return nil
}

// CopyFile efficiently copies a file from cache
func (cm *CacheManager) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	// Copy with buffer for better performance
	buffer := make([]byte, 64*1024) // 64KB buffer
	_, err = io.CopyBuffer(destFile, sourceFile, buffer)
	
	return err
}
