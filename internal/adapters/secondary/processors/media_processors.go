package processors

import (
	"context"
	"documents-worker/internal/core/ports"
	"documents-worker/media"
	"documents-worker/types"
	"fmt"
	"io"
	"os"
)

// VipsImageProcessor implements the ImageProcessor port using VIPS
type VipsImageProcessor struct{}

// NewVipsImageProcessor creates a new VIPS image processor
func NewVipsImageProcessor() ports.ImageProcessor {
	return &VipsImageProcessor{}
}

// Convert converts an image to the specified format
func (p *VipsImageProcessor) Convert(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error) {
	// Create temporary input file
	inputFile, err := os.CreateTemp("", "input-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(inputFile.Name())
	defer inputFile.Close()

	// Copy input to temp file
	_, err = io.Copy(inputFile, input)
	if err != nil {
		return nil, fmt.Errorf("failed to copy input: %w", err)
	}

	// Create media converter
	converter := &types.MediaConverter{
		Kind:   types.ImageKind,
		Format: &outputFormat,
		Search: types.MediaSearch{},
	}

	// Apply parameters
	if quality, ok := params["quality"].(int); ok {
		converter.Search.Quality = &quality
	}
	if width, ok := params["width"].(int); ok {
		converter.Search.Width = &width
	}
	if height, ok := params["height"].(int); ok {
		converter.Search.Height = &height
	}

	// Process with VIPS
	outputFile, err := media.ExecCommand(true, inputFile.Name(), converter)
	if err != nil {
		return nil, fmt.Errorf("failed to process image with VIPS: %w", err)
	}

	return outputFile, nil
}

// Resize resizes an image to the specified dimensions
func (p *VipsImageProcessor) Resize(ctx context.Context, input io.Reader, width, height int, params map[string]interface{}) (io.Reader, error) {
	resizeParams := make(map[string]interface{})
	for k, v := range params {
		resizeParams[k] = v
	}
	resizeParams["width"] = width
	resizeParams["height"] = height

	return p.Convert(ctx, input, "webp", resizeParams)
}

// GenerateThumbnail generates a thumbnail of the specified size
func (p *VipsImageProcessor) GenerateThumbnail(ctx context.Context, input io.Reader, size int) (io.Reader, error) {
	params := map[string]interface{}{
		"width":   size,
		"height":  size,
		"quality": 85,
	}
	return p.Convert(ctx, input, "webp", params)
}

// FFmpegVideoProcessor implements the VideoProcessor port using FFmpeg
type FFmpegVideoProcessor struct{}

// NewFFmpegVideoProcessor creates a new FFmpeg video processor
func NewFFmpegVideoProcessor() ports.VideoProcessor {
	return &FFmpegVideoProcessor{}
}

// Convert converts a video to the specified format
func (p *FFmpegVideoProcessor) Convert(ctx context.Context, input io.Reader, outputFormat string, params map[string]interface{}) (io.Reader, error) {
	// Create temporary input file
	inputFile, err := os.CreateTemp("", "input-*.mp4")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(inputFile.Name())
	defer inputFile.Close()

	// Copy input to temp file
	_, err = io.Copy(inputFile, input)
	if err != nil {
		return nil, fmt.Errorf("failed to copy input: %w", err)
	}

	// Create media converter
	converter := &types.MediaConverter{
		Kind:   types.VideoKind,
		Format: &outputFormat,
		Search: types.MediaSearch{},
	}

	// Apply parameters
	if quality, ok := params["quality"].(int); ok {
		converter.Search.Quality = &quality
	}
	if width, ok := params["width"].(int); ok {
		converter.Search.Width = &width
	}
	if height, ok := params["height"].(int); ok {
		converter.Search.Height = &height
	}

	// Process with FFmpeg
	outputFile, err := media.ExecCommand(false, inputFile.Name(), converter)
	if err != nil {
		return nil, fmt.Errorf("failed to process video with FFmpeg: %w", err)
	}

	return outputFile, nil
}

// GenerateThumbnail generates a thumbnail from a video at the specified time offset
func (p *FFmpegVideoProcessor) GenerateThumbnail(ctx context.Context, input io.Reader, timeOffset int) (io.Reader, error) {
	// Create temporary input file
	inputFile, err := os.CreateTemp("", "input-*.mp4")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(inputFile.Name())
	defer inputFile.Close()

	// Copy input to temp file
	_, err = io.Copy(inputFile, input)
	if err != nil {
		return nil, fmt.Errorf("failed to copy input: %w", err)
	}

	// Create media converter for thumbnail (use ImageKind for thumbnail output)
	converter := &types.MediaConverter{
		Kind:   types.ImageKind,
		Format: stringPtr("jpg"),
		Search: types.MediaSearch{
			Width:  intPtr(320),
			Height: intPtr(240),
		},
	}

	// Add time offset if specified
	if timeOffset > 0 {
		cutVideo := fmt.Sprintf("%d:1", timeOffset)
		converter.Search.CutVideo = &cutVideo
	}

	// Process with FFmpeg
	outputFile, err := media.ExecCommand(false, inputFile.Name(), converter)
	if err != nil {
		return nil, fmt.Errorf("failed to generate video thumbnail with FFmpeg: %w", err)
	}

	return outputFile, nil
}

// Compress compresses a video with the specified quality
func (p *FFmpegVideoProcessor) Compress(ctx context.Context, input io.Reader, quality int) (io.Reader, error) {
	params := map[string]interface{}{
		"quality": quality,
		"width":   854,
		"height":  480,
	}
	return p.Convert(ctx, input, "webm", params)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
