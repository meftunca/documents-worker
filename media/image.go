package media

import (
	"documents-worker/types"
	"fmt"
	"os"
)

type ImageProcessor struct {
	MediaConverter *types.MediaConverter
}

func (p *ImageProcessor) Process(inputPath string) (*os.File, error) {
	outputFile, err := ExecCommand(p.MediaConverter.VipsEnabled, inputPath, p.MediaConverter)
	if err != nil {
		return nil, fmt.Errorf("resim işleme hatası: %w", err)
	}
	return outputFile, nil
}
