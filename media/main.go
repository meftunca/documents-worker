package media

import (
	"documents-worker/types"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2/log"
)

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
	log.Infof("LibreOffice komutu: %s", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("LibreOffice Hatası: %v, Çıktı: %s", err, string(output))
		return "", err
	}
	pdfPath := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))+".pdf")
	return pdfPath, nil
}

func RunMutool(inputPath string, page int) (string, error) {
	outputFilePath := filepath.Join(os.TempDir(), "page.png")
	cmd := exec.Command("mutool", "draw", "-o", outputFilePath, "-r", "150", inputPath, strconv.Itoa(page))
	log.Infof("MuPDF komutu: %s", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("MuPDF Hatası: %v, Çıktı: %s", err, string(output))
		return "", err
	}
	return outputFilePath, nil
}
