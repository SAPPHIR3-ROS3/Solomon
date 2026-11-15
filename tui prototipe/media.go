package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"math"
	"strings"
)

//go:embed fixtures/preview.png
var previewPNG []byte

var previewImage image.Image
var previewError error
var imageRows = map[int]string{}

func imagePreview(width int) string {
	if cached, ok := imageRows[width]; ok {
		return cached
	}
	if previewImage == nil && previewError == nil {
		previewImage, previewError = png.Decode(bytes.NewReader(previewPNG))
	}
	if previewError != nil {
		return text("Image decode failed: "+previewError.Error(), "#FF5555")
	}
	output := []string{bold("Image attachment · icon.png", cyan), text(fmt.Sprintf("%d × %d · %d bytes · actual image pixels", previewImage.Bounds().Dx(), previewImage.Bounds().Dy(), len(previewPNG)), muted), renderRaster(previewImage, width, 72, 26)}
	result := strings.Join(output, "\n")
	imageRows[width] = result
	return result
}
func rasterPixel(picture image.Image, x, y, width, height int) [4]uint32 {
	b := picture.Bounds()
	sx := b.Min.X + min(b.Dx()-1, x*b.Dx()/width)
	sy := b.Min.Y + min(b.Dy()-1, y*b.Dy()/height)
	r, g, blue, a := picture.At(sx, sy).RGBA()
	if a == 0 {
		return [4]uint32{}
	}
	return [4]uint32{r * 255 / a, g * 255 / a, blue * 255 / a, a >> 8}
}

func renderRaster(picture image.Image, width, maxColumns, maxRows int) string {
	bounds := picture.Bounds()
	columns := min(max(1, width-2), maxColumns)
	rows := max(1, int(math.Round(float64(columns)*float64(bounds.Dy())/float64(bounds.Dx())/2)))
	if rows > maxRows {
		rows = maxRows
		columns = max(1, int(math.Round(float64(rows)*2*float64(bounds.Dx())/float64(bounds.Dy()))))
	}
	output := []string{}
	for y := 0; y < rows; y++ {
		var line strings.Builder
		for x := 0; x < columns; x++ {
			upper := rasterPixel(picture, x, y*2, columns, rows*2)
			lower := rasterPixel(picture, x, y*2+1, columns, rows*2)
			if upper[3] < 32 && lower[3] < 32 {
				line.WriteByte(' ')
				continue
			}
			if upper[3] < 32 {
				fmt.Fprintf(&line, "\x1b[38;2;%d;%d;%dm▄\x1b[0m", lower[0], lower[1], lower[2])
				continue
			}
			if lower[3] < 32 {
				fmt.Fprintf(&line, "\x1b[38;2;%d;%d;%dm▀\x1b[0m", upper[0], upper[1], upper[2])
				continue
			}
			fmt.Fprintf(&line, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀\x1b[0m", upper[0], upper[1], upper[2], lower[0], lower[1], lower[2])
		}
		output = append(output, line.String())
	}
	return strings.Join(output, "\n")
}
