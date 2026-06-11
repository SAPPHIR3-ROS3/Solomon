package tokcount

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"
)

func VisionTokensForFile(path string) int64 {
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 85
	}
	return visionTokens(cfg.Width, cfg.Height)
}

func VisionTokensForDataURL(url string) int64 {
	if !strings.HasPrefix(url, "data:") {
		return 0
	}
	comma := strings.Index(url, ",")
	if comma < 0 {
		return 0
	}
	raw, err := base64.StdEncoding.DecodeString(url[comma+1:])
	if err != nil || len(raw) == 0 {
		return 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return 85
	}
	return visionTokens(cfg.Width, cfg.Height)
}

func visionTokens(width, height int) int64 {
	if width <= 0 || height <= 0 {
		return 85
	}
	w, h := float64(width), float64(height)
	if w > 2048 || h > 2048 {
		scale := 2048 / math.Max(w, h)
		w *= scale
		h *= scale
	}
	short := math.Min(w, h)
	if short > 768 {
		scale := 768 / short
		w *= scale
		h *= scale
	}
	tilesW := int(math.Ceil(w / 512))
	tilesH := int(math.Ceil(h / 512))
	return int64(85 + 170*tilesW*tilesH)
}
