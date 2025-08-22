package media

import (
	"documents-worker/types"
	"fmt"
	"os"
)

type Processor interface {
	Process(inputPath string) (*os.File, error)
}

func NewProcessor(mediaConverter *types.MediaConverter) (Processor, error) {
	switch mediaConverter.Kind {
	case types.ImageKind:
		return &ImageProcessor{MediaConverter: mediaConverter}, nil
	case types.VideoKind:
		return &VideoProcessor{MediaConverter: mediaConverter}, nil
	case types.DocKind:
		return &DocumentProcessor{MediaConverter: mediaConverter}, nil
	default:
		return nil, fmt.Errorf("bilinmeyen medya türü: %s", mediaConverter.Kind)
	}
}
