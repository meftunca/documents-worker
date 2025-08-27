package media

import (
	"context"
	"documents-worker/pkg/memory"
	"documents-worker/types"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2/log"
	"github.com/rs/zerolog"
)

var (
	memoryManager *memory.Manager
	logger        zerolog.Logger
)

// Initialize memory manager for media processing
func init() {
	memoryManager = memory.NewManager(nil)
	logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

// ProcessingOptions defines options for media processing
type ProcessingOptions struct {
	UseStreaming bool
	BufferSize   int
	EnableCache  bool
}

// OptimizedExecCommand processes media files with memory optimization
func OptimizedExecCommand(vipsEnabled bool, inputPath string, m *types.MediaConverter, opts *ProcessingOptions) (*os.File, error) {
	if opts == nil {
		opts = &ProcessingOptions{
			UseStreaming: true,
			BufferSize:   64 * 1024, // 64KB buffer
			EnableCache:  true,
		}
	}

	// Get memory pool for processing
	poolConfig := memory.DefaultPoolConfig()
	poolConfig.BufferSize = opts.BufferSize
	pool, err := memoryManager.GetPool("media-processing", poolConfig)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get memory pool")
		return ExecCommand(vipsEnabled, inputPath, m) // Fallback to original
	}

	ctx := context.Background()
	buffer, err := pool.Acquire(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to acquire buffer")
		return ExecCommand(vipsEnabled, inputPath, m) // Fallback to original
	}
	defer buffer.Release()

	var extension string
	if m.Kind == types.ImageKind {
		if m.Format != nil {
			extension = *m.Format
		} else {
			extension = "webp"
		}
	} else if m.Kind == types.VideoKind {
		extension = "webm"
	} else {
		return nil, fmt.Errorf("bilinmeyen medya türü için çıktı formatı belirlenemedi: %s", m.Kind)
	}

	outputFile, err := os.CreateTemp("", fmt.Sprintf("processed-*.%s", extension))
	if err != nil {
		return nil, fmt.Errorf("geçici çıktı dosyası oluşturulamadı: %w", err)
	}
	defer outputFile.Close()

	var cmd *exec.Cmd
	if vipsEnabled && m.Kind == types.ImageKind {
		args := buildVipsArgs(inputPath, outputFile.Name(), m)
		cmd = exec.Command("vips", args...)
	} else {
		args := buildFFmpegArgs(inputPath, outputFile.Name(), m)
		cmd = exec.Command("ffmpeg", args...)
	}

	// Use streaming processing for large files
	if opts.UseStreaming {
		return executeWithStreaming(cmd, outputFile, buffer)
	}

	logger.Info().Str("command", cmd.String()).Msg("Executing media processing command")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().Err(err).Str("output", string(output)).Msg("Command execution failed")
		return nil, fmt.Errorf("komut çalıştırma hatası: %w", err)
	}

	return os.OpenFile(outputFile.Name(), os.O_RDONLY, 0666)
}

// executeWithStreaming executes command with streaming I/O
func executeWithStreaming(cmd *exec.Cmd, outputFile *os.File, buffer *memory.Buffer) (*os.File, error) {
	logger.Info().Str("command", cmd.String()).Msg("Executing command with streaming")

	// Start the command
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Wait for completion
	err = cmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("command execution failed: %w", err)
	}

	return os.OpenFile(outputFile.Name(), os.O_RDONLY, 0666), nil
}

// ExecCommand, belirlenen işleyiciyi (VIPS veya FFMPEG) çalıştıran ana fonksiyondur.
func ExecCommand(vipsEnabled bool, inputPath string, m *types.MediaConverter) (*os.File, error) {
	var cmd *exec.Cmd
	var extension string

	if m.Kind == types.ImageKind {
		if m.Format != nil {
			extension = *m.Format
		} else {
			extension = "webp"
		}
	} else if m.Kind == types.VideoKind {
		extension = "webm"
	} else {
		return nil, fmt.Errorf("bilinmeyen medya türü için çıktı formatı belirlenemedi: %s", m.Kind)
	}

	outputFile, err := os.CreateTemp("", fmt.Sprintf("processed-*.%s", extension))
	if err != nil {
		return nil, fmt.Errorf("geçici çıktı dosyası oluşturulamadı: %w", err)
	}
	defer outputFile.Close()

	if vipsEnabled && m.Kind == types.ImageKind {
		args := buildVipsArgs(inputPath, outputFile.Name(), m)
		cmd = exec.Command("vips", args...)
	} else {
		args := buildFFmpegArgs(inputPath, outputFile.Name(), m)
		cmd = exec.Command("ffmpeg", args...)
	}

	log.Infof("Komut çalıştırılıyor: %s", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("Komut Hatası: %v, Çıktı: %s", err, string(output))
		return nil, fmt.Errorf("komut çalıştırma hatası: %w", err)
	}

	return os.OpenFile(outputFile.Name(), os.O_RDONLY, 0666)
}

// StreamingProcessor handles large file processing with memory efficiency
type StreamingProcessor struct {
	buffer    *memory.Buffer
	tempDir   string
	chunkSize int
	logger    zerolog.Logger
}

// NewStreamingProcessor creates a new streaming processor
func NewStreamingProcessor(bufferSize int) (*StreamingProcessor, error) {
	poolConfig := memory.DefaultPoolConfig()
	poolConfig.BufferSize = bufferSize

	pool, err := memoryManager.GetPool("streaming", poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory pool: %w", err)
	}

	ctx := context.Background()
	buffer, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire buffer: %w", err)
	}

	tempDir := os.TempDir()
	return &StreamingProcessor{
		buffer:    buffer,
		tempDir:   tempDir,
		chunkSize: bufferSize,
		logger:    logger.With().Str("component", "streaming-processor").Logger(),
	}, nil
}

// ProcessLargeFile processes large files in chunks
func (sp *StreamingProcessor) ProcessLargeFile(inputPath string, processor func(chunk []byte) error) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	sp.logger.Info().Int64("file_size", info.Size()).Str("file", inputPath).Msg("Starting streaming processing")

	buffer := sp.buffer.Data()
	totalProcessed := int64(0)

	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Process chunk
		if err := processor(buffer[:n]); err != nil {
			return fmt.Errorf("failed to process chunk: %w", err)
		}

		totalProcessed += int64(n)

		// Log progress for large files
		if totalProcessed%int64(sp.chunkSize*100) == 0 {
			progress := float64(totalProcessed) / float64(info.Size()) * 100
			sp.logger.Info().Float64("progress", progress).Msg("Processing progress")
		}
	}

	sp.logger.Info().Int64("total_processed", totalProcessed).Msg("Streaming processing completed")
	return nil
}

// Close releases resources
func (sp *StreamingProcessor) Close() error {
	if sp.buffer != nil {
		return sp.buffer.Release()
	}
	return nil
}

// ConcurrentProcessor handles parallel processing
type ConcurrentProcessor struct {
	workerCount int
	bufferPool  *memory.Pool
	logger      zerolog.Logger
}

// NewConcurrentProcessor creates a concurrent processor
func NewConcurrentProcessor(workerCount int) (*ConcurrentProcessor, error) {
	poolConfig := memory.DefaultPoolConfig()
	poolConfig.InitialBuffers = workerCount
	poolConfig.MaxBuffers = workerCount * 2

	pool, err := memoryManager.GetPool("concurrent", poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory pool: %w", err)
	}

	return &ConcurrentProcessor{
		workerCount: workerCount,
		bufferPool:  pool,
		logger:      logger.With().Str("component", "concurrent-processor").Logger(),
	}, nil
}

// ProcessBatch processes multiple files concurrently
func (cp *ConcurrentProcessor) ProcessBatch(files []string, processor func(string) error) error {
	jobs := make(chan string, len(files))
	results := make(chan error, len(files))

	// Start workers
	for i := 0; i < cp.workerCount; i++ {
		go cp.worker(jobs, results, processor)
	}

	// Send jobs
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Collect results
	var errors []error
	for i := 0; i < len(files); i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch processing failed with %d errors: %v", len(errors), errors[0])
	}

	return nil
}

// worker processes jobs from the job channel
func (cp *ConcurrentProcessor) worker(jobs <-chan string, results chan<- error, processor func(string) error) {
	for file := range jobs {
		cp.logger.Info().Str("file", file).Msg("Processing file")
		err := processor(file)
		results <- err
	}
}

// MemoryStats returns current memory usage statistics
func MemoryStats() map[string]interface{} {
	if memoryManager == nil {
		return map[string]interface{}{"error": "memory manager not initialized"}
	}
	return memoryManager.MemoryUsage()
}

// CleanupMemory forces cleanup of memory pools
func CleanupMemory() {
	if memoryManager != nil {
		// Force cleanup of all pools
		stats := memoryManager.GetTotalStats()
		logger.Info().Interface("stats", stats).Msg("Memory cleanup requested")
	}
}

// Keep original functions for backward compatibility
func buildVipsArgs(inputPath string, outputPath string, m *types.MediaConverter) []string {
	outputWithOpts := outputPath
	if m.Search.Quality != nil {
		outputWithOpts = fmt.Sprintf("%s[Q=%d]", outputPath, *m.Search.Quality)
	}
	if m.Search.ResizeScale != nil {
		scaleFactor := float64(*m.Search.ResizeScale) / 100.0
		return []string{"resize", inputPath, outputWithOpts, fmt.Sprintf("%f", scaleFactor)}
	} else if m.Search.Crop != nil {
		parts := strings.Split(*m.Search.Crop, ":")
		return []string{"extract_area", inputPath, outputWithOpts, parts[0], parts[1], parts[2], parts[3]}
	} else if m.Search.Width != nil || m.Search.Height != nil {
		width := "1"
		if m.Search.Width != nil {
			width = strconv.Itoa(*m.Search.Width)
		}
		args := []string{"thumbnail", inputPath, outputWithOpts, width}
		if m.Search.Height != nil {
			args = append(args, "--height", strconv.Itoa(*m.Search.Height))
		}
		return args
	} else {
		return []string{"copy", inputPath, outputWithOpts}
	}
}

func buildFFmpegArgs(inputPath string, outputPath string, m *types.MediaConverter) []string {
	args := []string{"-i", inputPath}
	if m.Kind == types.ImageKind {
		vf := []string{}
		if m.Search.ResizeScale != nil {
			vf = append(vf, fmt.Sprintf("scale=iw*%d/100:ih*%d/100", *m.Search.ResizeScale, *m.Search.ResizeScale))
		} else if m.Search.Width != nil || m.Search.Height != nil {
			w, h := "-1", "-1"
			if m.Search.Width != nil {
				w = strconv.Itoa(*m.Search.Width)
			}
			if m.Search.Height != nil {
				h = strconv.Itoa(*m.Search.Height)
			}
			vf = append(vf, fmt.Sprintf("scale=%s:%s", w, h))
		}
		if m.Search.Crop != nil {
			vf = append(vf, fmt.Sprintf("crop=%s", *m.Search.Crop))
		}
		if len(vf) > 0 {
			args = append(args, "-vf", strings.Join(vf, ","))
		}
		if m.Search.Quality != nil {
			q := 31 - (*m.Search.Quality * 30 / 100)
			args = append(args, "-q:v", strconv.Itoa(q))
		}
		if m.Format != nil && *m.Format == "avif" {
			args = append(args, "-c:v", "libaom-av1", "-still-picture", "1")
		}
	} else if m.Kind == types.VideoKind && m.Search.CutVideo != nil {
		parts := strings.Split(*m.Search.CutVideo, ":")
		if len(parts) == 2 {
			args = append(args, "-ss", parts[0], "-t", parts[1])
		}
	}
	args = append(args, "-y", outputPath)
	return args
}

func RunLibreOffice(inputPath string) (string, error) {
	outputDir := os.TempDir()
	cmd := exec.Command("soffice", "--headless", "--convert-to", "pdf", inputPath, "--outdir", outputDir)
	logger.Info().Str("command", cmd.String()).Msg("LibreOffice command")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().Err(err).Str("output", string(output)).Msg("LibreOffice error")
		return "", err
	}
	pdfPath := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))+".pdf")
	return pdfPath, nil
}

func RunMutool(inputPath string, page int) (string, error) {
	outputFilePath := filepath.Join(os.TempDir(), "page.png")
	cmd := exec.Command("mutool", "draw", "-o", outputFilePath, "-r", "150", inputPath, strconv.Itoa(page))
	logger.Info().Str("command", cmd.String()).Msg("MuPDF command")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().Err(err).Str("output", string(output)).Msg("MuPDF error")
		return "", err
	}
	return outputFilePath, nil
}
