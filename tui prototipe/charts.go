package main

import (
	"bytes"
	"embed"
	"fmt"
	"image/png"
)

//go:embed fixtures/pie.png fixtures/sankey.png fixtures/radar.png fixtures/pie.svg fixtures/sankey.svg fixtures/radar.svg
var chartImages embed.FS

var chartRows = map[string]string{}

func mermaidImage(kind string, width int) string {
	key := fmt.Sprintf("%s:%d", kind, width)
	if cached, ok := chartRows[key]; ok {
		return cached
	}
	data, err := chartImages.ReadFile("fixtures/" + kind + ".png")
	if err != nil {
		return text("Mermaid image unavailable: "+err.Error(), "#FF5555")
	}
	picture, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return text("Mermaid decode failed: "+err.Error(), "#FF5555")
	}
	maxRows := 38
	if kind == "radar" {
		maxRows = 34
	}
	result := renderRaster(picture, width, 120, maxRows)
	geometry, err := chartImages.ReadFile("fixtures/" + kind + ".svg")
	if err != nil {
		return text("Mermaid labels unavailable: "+err.Error(), "#FF5555")
	}
	result, err = overlayChartLabels(result, geometry, picture)
	if err != nil {
		return text("Mermaid labels failed: "+err.Error(), "#FF5555")
	}
	chartRows[key] = result
	return result
}
