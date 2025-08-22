package media

import (
	"documents-worker/types"
	"documents-worker/utils"
	"fmt"
	"os"
)

type DocumentProcessor struct {
	MediaConverter *types.MediaConverter
}

func (p *DocumentProcessor) Process(inputPath string) (*os.File, error) {
	currentPath := inputPath

	mimeType, err := utils.DetectMimeTypeFromFile(currentPath)
	if err != nil {
		return nil, err
	}

	// Adım 1: Office belgesi ise PDF'e dönüştür
	if utils.IsOfficeDocument(mimeType) {
		currentPath, err = RunLibreOffice(currentPath)
		if err != nil {
			return nil, fmt.Errorf("libreoffice dönüştürme hatası: %w", err)
		}
		defer os.Remove(currentPath)
	}

	// Adım 2: PDF ise resme dönüştür
	if utils.IsPdfDocument(mimeType) && (p.MediaConverter.Format == nil || *p.MediaConverter.Format != "pdf") {
		page := 1
		if p.MediaConverter.Search.Page != nil {
			page = *p.MediaConverter.Search.Page
		}
		currentPath, err = RunMutool(currentPath, page)
		if err != nil {
			return nil, fmt.Errorf("mutool ile sayfa çıkarma hatası: %w", err)
		}
		defer os.Remove(currentPath)
	}

	// Doküman işlendikten sonra ImageProcessor'a devret
	imageProcessor := ImageProcessor{MediaConverter: p.MediaConverter}
	return imageProcessor.Process(currentPath)
}
