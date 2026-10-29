// Package imagepreview renders static terminal previews without external codecs
// or rendering libraries. The original media remains the source of truth.
package imagepreview

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
)

const MaxBytes = 32 << 20
const MaxPixels = 32 << 20

// Decode bounds allocations before decoding. image/gif.Decode reads only the
// first image; its logical canvas and image offset are retained by canvasImage.
func Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("preview exceeds %d MiB", MaxBytes>>20)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode preview header: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxPixels/cfg.Height {
		return nil, fmt.Errorf("preview exceeds %d pixels", MaxPixels)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode preview: %w", err)
	}
	return canvasImage{Image: img, bounds: image.Rect(0, 0, cfg.Width, cfg.Height)}, nil
}

func Open(path string) (image.Image, error) {
	before, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("preview is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("preview is not a regular file")
	}
	return Decode(f)
}

type canvasImage struct {
	image.Image
	bounds image.Rectangle
}

func (c canvasImage) Bounds() image.Rectangle { return c.bounds }
