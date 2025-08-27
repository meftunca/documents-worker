package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client represents the documents worker client
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// Config holds client configuration
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// Response represents API responses
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	JobID   string      `json:"job_id,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JobStatus represents job status response
type JobStatus struct {
	ID          string      `json:"id"`
	Status      string      `json:"status"`
	Progress    int         `json:"progress"`
	Result      interface{} `json:"result,omitempty"`
	Error       string      `json:"error,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}

// ProcessingOptions for document processing
type ProcessingOptions struct {
	Format   string            `json:"format,omitempty"`
	Quality  int               `json:"quality,omitempty"`
	Width    int               `json:"width,omitempty"`
	Height   int               `json:"height,omitempty"`
	Page     int               `json:"page,omitempty"`
	Language string            `json:"language,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewClient creates a new documents worker client
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Client{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ProcessDocument processes a document file
func (c *Client) ProcessDocument(filePath string, options *ProcessingOptions) (*Response, error) {
	return c.uploadFile(filePath, "/process/document", options)
}

// ProcessImage processes an image file
func (c *Client) ProcessImage(filePath string, options *ProcessingOptions) (*Response, error) {
	return c.uploadFile(filePath, "/process/image", options)
}

// ProcessVideo processes a video file
func (c *Client) ProcessVideo(filePath string, options *ProcessingOptions) (*Response, error) {
	return c.uploadFile(filePath, "/process/video", options)
}

// ExtractText extracts text from a document
func (c *Client) ExtractText(filePath string, options *ProcessingOptions) (*Response, error) {
	return c.uploadFile(filePath, "/extract/text", options)
}

// PerformOCR performs OCR on an image or document
func (c *Client) PerformOCR(filePath string, options *ProcessingOptions) (*Response, error) {
	return c.uploadFile(filePath, "/ocr/process", options)
}

// ChunkDocument chunks a document for RAG applications
func (c *Client) ChunkDocument(filePath string, options *ProcessingOptions) (*Response, error) {
	return c.uploadFile(filePath, "/chunk/document", options)
}

// GetJobStatus gets the status of a job
func (c *Client) GetJobStatus(jobID string) (*JobStatus, error) {
	url := fmt.Sprintf("%s/jobs/%s/status", c.baseURL, jobID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var jobStatus JobStatus
	if err := json.NewDecoder(resp.Body).Decode(&jobStatus); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &jobStatus, nil
}

// WaitForJob waits for a job to complete
func (c *Client) WaitForJob(jobID string, pollInterval time.Duration) (*JobStatus, error) {
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	for {
		status, err := c.GetJobStatus(jobID)
		if err != nil {
			return nil, err
		}

		switch status.Status {
		case "completed", "success":
			return status, nil
		case "failed", "error":
			return status, fmt.Errorf("job failed: %s", status.Error)
		case "pending", "processing", "in_progress":
			time.Sleep(pollInterval)
			continue
		default:
			return status, nil
		}
	}
}

// GetJobResult gets the result of a completed job
func (c *Client) GetJobResult(jobID string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/jobs/%s/result", c.baseURL, jobID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	return resp.Body, nil
}

// Health checks the health of the service
func (c *Client) Health() (*Response, error) {
	url := fmt.Sprintf("%s/health", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// uploadFile uploads a file with optional processing options
func (c *Client) uploadFile(filePath, endpoint string, options *ProcessingOptions) (*Response, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	// Add options if provided
	if options != nil {
		if options.Format != "" {
			writer.WriteField("format", options.Format)
		}
		if options.Quality > 0 {
			writer.WriteField("quality", fmt.Sprintf("%d", options.Quality))
		}
		if options.Width > 0 {
			writer.WriteField("width", fmt.Sprintf("%d", options.Width))
		}
		if options.Height > 0 {
			writer.WriteField("height", fmt.Sprintf("%d", options.Height))
		}
		if options.Page > 0 {
			writer.WriteField("page", fmt.Sprintf("%d", options.Page))
		}
		if options.Language != "" {
			writer.WriteField("language", options.Language)
		}

		// Add metadata
		for key, value := range options.Metadata {
			writer.WriteField(fmt.Sprintf("metadata[%s]", key), value)
		}
	}

	writer.Close()

	// Create request
	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return &response, fmt.Errorf("API error: %s", response.Error)
	}

	return &response, nil
}

// ProcessAndWait processes a file and waits for completion
func (c *Client) ProcessAndWait(filePath, operation string, options *ProcessingOptions) (*JobStatus, error) {
	var resp *Response
	var err error

	switch operation {
	case "document":
		resp, err = c.ProcessDocument(filePath, options)
	case "image":
		resp, err = c.ProcessImage(filePath, options)
	case "video":
		resp, err = c.ProcessVideo(filePath, options)
	case "text":
		resp, err = c.ExtractText(filePath, options)
	case "ocr":
		resp, err = c.PerformOCR(filePath, options)
	case "chunk":
		resp, err = c.ChunkDocument(filePath, options)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	if err != nil {
		return nil, err
	}

	if resp.JobID == "" {
		return nil, fmt.Errorf("no job ID returned")
	}

	return c.WaitForJob(resp.JobID, 2*time.Second)
}

// BatchProcess processes multiple files concurrently
func (c *Client) BatchProcess(files []string, operation string, options *ProcessingOptions, maxConcurrency int) ([]*JobStatus, error) {
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}

	jobs := make(chan string, len(files))
	results := make(chan *JobStatus, len(files))
	errors := make(chan error, len(files))

	// Start workers
	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for filePath := range jobs {
				status, err := c.ProcessAndWait(filePath, operation, options)
				if err != nil {
					errors <- err
				} else {
					results <- status
				}
			}
		}()
	}

	// Send jobs
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Collect results
	var jobStatuses []*JobStatus
	var processErrors []error

	for i := 0; i < len(files); i++ {
		select {
		case status := <-results:
			jobStatuses = append(jobStatuses, status)
		case err := <-errors:
			processErrors = append(processErrors, err)
		}
	}

	if len(processErrors) > 0 {
		return jobStatuses, fmt.Errorf("batch processing had %d errors: %v", len(processErrors), processErrors[0])
	}

	return jobStatuses, nil
}
