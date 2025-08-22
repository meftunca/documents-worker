package media

import (
	"documents-worker/types"
	"os"
)

type VideoProcessor struct {
	MediaConverter *types.MediaConverter
}

func (p *VideoProcessor) Process(inputPath string) (*os.File, error) {
	// Video için VIPS devre dışı bırakılır, her zaman FFMPEG kullanılır.
	outputFile, err := ExecCommand(false, inputPath, p.MediaConverter)
	if err != nil {
		return nil, err
	}
	return outputFile, nil
}
