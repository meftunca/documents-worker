package cache

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileCache provides file-based caching functionality
type FileCache struct {
	directory  string
	ttl        time.Duration
	maxSize    int64
	cleanupAge time.Duration
}

// NewFileCache creates a new file cache instance
func NewFileCache(directory string, ttl time.Duration, maxSize int64, cleanupAge time.Duration) *FileCache {
	// Create cache directory if it doesn't exist
	os.MkdirAll(directory, 0755)

	return &FileCache{
		directory:  directory,
		ttl:        ttl,
		maxSize:    maxSize,
		cleanupAge: cleanupAge,
	}
}

// GenerateCacheKey generates a cache key based on input parameters
func (fc *FileCache) GenerateCacheKey(operation string, params ...string) string {
	h := md5.New()
	h.Write([]byte(operation))
	for _, param := range params {
		h.Write([]byte(param))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Get retrieves a cached file
func (fc *FileCache) Get(key string) (string, bool) {
	cachePath := filepath.Join(fc.directory, key)

	// Check if file exists
	info, err := os.Stat(cachePath)
	if err != nil {
		return "", false
	}

	// Check if file is still valid (TTL)
	if time.Since(info.ModTime()) > fc.ttl {
		os.Remove(cachePath)
		return "", false
	}

	return cachePath, true
}

// Set stores a file in cache
func (fc *FileCache) Set(key string, sourcePath string) error {
	cachePath := filepath.Join(fc.directory, key)

	// Copy source file to cache
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	_, err = io.Copy(cacheFile, sourceFile)
	return err
}

// Delete removes a file from cache
func (fc *FileCache) Delete(key string) error {
	cachePath := filepath.Join(fc.directory, key)
	return os.Remove(cachePath)
}

// Clean removes expired cache files
func (fc *FileCache) Clean() error {
	return filepath.Walk(fc.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && time.Since(info.ModTime()) > fc.cleanupAge {
			os.Remove(path)
		}

		return nil
	})
}

// Size returns the total size of the cache directory
func (fc *FileCache) Size() (int64, error) {
	var size int64

	err := filepath.Walk(fc.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// EnforceMaxSize removes oldest files if cache exceeds max size
func (fc *FileCache) EnforceMaxSize() error {
	currentSize, err := fc.Size()
	if err != nil {
		return err
	}

	if currentSize <= fc.maxSize {
		return nil
	}

	// Get all files with their modification times
	type fileInfo struct {
		path    string
		modTime time.Time
		size    int64
	}

	var files []fileInfo

	err = filepath.Walk(fc.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, fileInfo{
				path:    path,
				modTime: info.ModTime(),
				size:    info.Size(),
			})
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Sort by modification time (oldest first)
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.After(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Remove oldest files until under max size
	for _, file := range files {
		if currentSize <= fc.maxSize {
			break
		}

		if err := os.Remove(file.path); err == nil {
			currentSize -= file.size
		}
	}

	return nil
}
