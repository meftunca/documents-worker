package client

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-key",
		Timeout: 10 * time.Second,
	}

	client := NewClient(config)

	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.baseURL != "http://localhost:8080" {
		t.Errorf("Expected baseURL to be 'http://localhost:8080', got '%s'", client.baseURL)
	}

	if client.apiKey != "test-key" {
		t.Errorf("Expected apiKey to be 'test-key', got '%s'", client.apiKey)
	}

	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("Expected timeout to be 10s, got %v", client.httpClient.Timeout)
	}
}

func TestNewClientWithDefaults(t *testing.T) {
	config := Config{
		BaseURL: "http://localhost:8080",
	}

	client := NewClient(config)

	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout to be 30s, got %v", client.httpClient.Timeout)
	}
}

func TestProcessingOptions(t *testing.T) {
	options := &ProcessingOptions{
		Format:   "webp",
		Quality:  85,
		Width:    800,
		Height:   600,
		Page:     1,
		Language: "eng",
		Metadata: map[string]string{
			"source": "test",
			"type":   "document",
		},
	}

	if options.Format != "webp" {
		t.Errorf("Expected format to be 'webp', got '%s'", options.Format)
	}

	if options.Quality != 85 {
		t.Errorf("Expected quality to be 85, got %d", options.Quality)
	}

	if options.Width != 800 {
		t.Errorf("Expected width to be 800, got %d", options.Width)
	}

	if options.Height != 600 {
		t.Errorf("Expected height to be 600, got %d", options.Height)
	}

	if options.Page != 1 {
		t.Errorf("Expected page to be 1, got %d", options.Page)
	}

	if options.Language != "eng" {
		t.Errorf("Expected language to be 'eng', got '%s'", options.Language)
	}

	if options.Metadata["source"] != "test" {
		t.Errorf("Expected metadata source to be 'test', got '%s'", options.Metadata["source"])
	}

	if options.Metadata["type"] != "document" {
		t.Errorf("Expected metadata type to be 'document', got '%s'", options.Metadata["type"])
	}
}

// Benchmark tests
func BenchmarkClientCreation(b *testing.B) {
	config := Config{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := NewClient(config)
		_ = client
	}
}

func BenchmarkProcessingOptionsCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		options := &ProcessingOptions{
			Format:   "webp",
			Quality:  85,
			Width:    800,
			Height:   600,
			Language: "eng",
			Metadata: map[string]string{
				"source": "benchmark",
				"test":   "true",
			},
		}
		_ = options
	}
}
