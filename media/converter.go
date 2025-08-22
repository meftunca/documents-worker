package media

import (
	"documents-worker/types"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

func NewMediaConverterFromFiber(c *fiber.Ctx) (*types.MediaConverter, error) {
	media := &types.MediaConverter{
		VipsEnabled: true,
	}

	if c.Query("vipsEnable") == "false" {
		media.VipsEnabled = false
		log.Info("VIPS processing is disabled by query parameter.")
	}
	if format := c.Query("format"); format != "" {
		media.Format = &format
	}
	if width := c.Query("width"); width != "" {
		w, _ := strconv.Atoi(width)
		media.Search.Width = &w
	}
	if height := c.Query("height"); height != "" {
		h, _ := strconv.Atoi(height)
		media.Search.Height = &h
	}
	if crop := c.Query("crop"); crop != "" {
		media.Search.Crop = &crop
	}
	if quality := c.Query("quality"); quality != "" {
		q, _ := strconv.Atoi(quality)
		media.Search.Quality = &q
	}
	if resizeScale := c.Query("resize"); resizeScale != "" {
		r, _ := strconv.Atoi(resizeScale)
		media.Search.ResizeScale = &r
	}
	if cutVideo := c.Query("clip"); cutVideo != "" {
		media.Search.CutVideo = &cutVideo
	}
	if page := c.Query("page"); page != "" {
		p, _ := strconv.Atoi(page)
		if p > 0 {
			media.Search.Page = &p
		}
	}
	return media, nil
}
