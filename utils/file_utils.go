package utils

import (
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// SaveUploadedFile, yüklenen bir dosyayı geçici bir konuma kaydeder ve bir *os.File nesnesi döner.
func SaveUploadedFile(fileHeader *multipart.FileHeader) (*os.File, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, src); err != nil {
		os.Remove(tempFile.Name())
		return nil, err
	}

	return os.OpenFile(tempFile.Name(), os.O_RDONLY, 0666)
}

// DetectMimeTypeFromFile, verilen dosya yolundaki dosyanın MIME türünü algılar.
func DetectMimeTypeFromFile(filePath string) (string, error) {
	mime, err := mimetype.DetectFile(filePath)
	if err != nil {
		return "", err
	}
	return mime.String(), nil
}

// IsOfficeDocument, verilen MIME türünün bir Office belgesi olup olmadığını kontrol eder.
func IsOfficeDocument(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	officeFormats := []string{
		"application/vnd.openxmlformats-officedocument",
		"application/vnd.ms-",
		"application/msword",
		"application/vnd.oasis.opendocument",
	}
	for _, format := range officeFormats {
		if strings.Contains(mimeType, format) {
			return true
		}
	}
	return false
}

// IsPdfDocument, verilen MIME türünün bir PDF belgesi olup olmadığını kontrol eder.
func IsPdfDocument(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	return strings.Contains(mimeType, "pdf")
}

// IsImageFile, verilen MIME türünün bir resim dosyası olup olmadığını kontrol eder.
func IsImageFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// IsVideoFile, verilen MIME türünün bir video dosyası olup olmadığını kontrol eder.
func IsVideoFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}

// IsDocumentFile, genel bir doküman türü kontrolü yapar (PDF veya Office).
func IsDocumentFile(mimeType string) bool {
	return IsOfficeDocument(mimeType) || IsPdfDocument(mimeType)
}
