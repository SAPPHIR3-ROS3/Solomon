package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type chartPosition struct {
	x, y   float64
	anchor string
	legend bool
	sankey bool
}

type chartLabel struct {
	chartPosition
	value string
	slice bool
}

func chartNumber(value string) float64 {
	number, _ := strconv.ParseFloat(value, 64)
	return number
}

func chartGeometry(source []byte) (float64, float64, []chartLabel, error) {
	decoder := xml.NewDecoder(bytes.NewReader(source))
	stack := []chartPosition{{}}
	var labels []chartLabel
	var label *chartLabel
	var width, height float64
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, nil, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			position := stack[len(stack)-1]
			attributes := map[string]string{}
			for _, attribute := range item.Attr {
				attributes[attribute.Name.Local] = attribute.Value
			}
			if item.Name.Local == "svg" {
				view := strings.Fields(attributes["viewBox"])
				if len(view) == 4 {
					width, height = chartNumber(view[2]), chartNumber(view[3])
					position.x, position.y = -chartNumber(view[0]), -chartNumber(view[1])
				}
			}
			transform := strings.TrimSpace(attributes["transform"])
			if strings.HasPrefix(transform, "translate(") {
				values := strings.Fields(strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(transform, "translate("), ")"), ",", " "))
				if len(values) > 0 {
					position.x += chartNumber(values[0])
				}
				if len(values) > 1 {
					position.y += chartNumber(values[1])
				}
			} else if transform != "" {
				return 0, 0, nil, fmt.Errorf("unsupported chart transform: %s", transform)
			}
			if anchor := attributes["text-anchor"]; anchor != "" {
				position.anchor = anchor
			}
			if attributes["class"] == "node-labels" {
				position.sankey = true
			}
			if attributes["class"] == "legend" {
				position.legend = true
			}
			if item.Name.Local == "text" {
				position.x += chartNumber(attributes["x"])
				position.y += chartNumber(attributes["y"])
				class := attributes["class"]
				if class == "radarLegendText" {
					position.legend = true
				}
				if strings.Contains(class, "Title") || class == "slice" || strings.Contains(attributes["style"], "text-anchor: middle") {
					position.anchor = "middle"
				}
				label = &chartLabel{chartPosition: position, slice: class == "slice"}
			}
			stack = append(stack, position)
		case xml.CharData:
			if label != nil {
				label.value += string(item)
			}
		case xml.EndElement:
			if item.Name.Local == "text" && label != nil {
				label.value = strings.Join(strings.Fields(label.value), " ")
				if label.value != "" {
					labels = append(labels, *label)
				}
				label = nil
			}
			stack = stack[:len(stack)-1]
		}
	}
	if width <= 0 || height <= 0 {
		return 0, 0, nil, fmt.Errorf("invalid chart viewBox")
	}
	return width, height, labels, nil
}

func overlayChartLabels(raster string, source []byte, pictures ...image.Image) (string, error) {
	width, height, labels, err := chartGeometry(source)
	if err != nil {
		return "", err
	}
	rows := strings.Split(raster, "\n")
	columns := ansi.StringWidth(rows[0])
	legendRow, legendIndex := -1, 0
	for _, label := range labels {
		value := ansi.Truncate(label.value, columns, "")
		length := ansi.StringWidth(value)
		x := int(math.Round(label.x / width * float64(columns)))
		y := int(math.Round(label.y / height * float64(len(rows))))
		switch label.anchor {
		case "middle":
			x -= length / 2
		case "end":
			x -= length
		}
		if label.legend {
			if legendRow < 0 {
				legendRow = y
			}
			y = legendRow + legendIndex*2
			legendIndex++
			x -= 3
			value = "■ " + value
			length = ansi.StringWidth(value)
		}
		x = max(0, min(x, columns-length))
		y = max(0, min(y, len(rows)-1))
		styled := text(value, ink)
		if label.legend {
			palette := []string{"#3b8fd1", "#ffc704", "#e8e5df"}
			styled = text("■", palette[(legendIndex-1)%len(palette)]) + text(" "+strings.TrimPrefix(value, "■ "), ink)
		}
		if label.slice && len(pictures) > 0 {
			pixel := rasterPixel(pictures[0], min(columns-1, x+length/2), y*2, columns, len(rows)*2)
			styled = fmt.Sprintf("\x1b[38;2;6;28;59;48;2;%d;%d;%dm%s\x1b[0m", pixel[0], pixel[1], pixel[2], value)
		}
		if label.sankey && len(pictures) > 0 {
			styled = chartLabelOnRaster(value, pictures[0], x, y, columns, len(rows))
		}
		rows[y] = ansi.Cut(rows[y], 0, x) + styled + ansi.Cut(rows[y], x+length, columns)
	}
	return strings.Join(rows, "\n"), nil
}

func chartLabelOnRaster(value string, picture image.Image, x, y, columns, rows int) string {
	var out strings.Builder
	column := x
	for _, character := range value {
		glyph := string(character)
		pixel := rasterPixel(picture, min(columns-1, column), y*2, columns, rows*2)
		if pixel[3] < 32 {
			out.WriteString(text(glyph, ink))
		} else {
			red, green, blue := pixel[0], pixel[1], pixel[2]
			foreground := "232;229;223"
			if 299*red+587*green+114*blue > 140000 {
				foreground = "6;28;59"
			}
			fmt.Fprintf(&out, "\x1b[38;2;%s;48;2;%d;%d;%dm%s\x1b[0m", foreground, red, green, blue, glyph)
		}
		column += ansi.StringWidth(glyph)
	}
	return out.String()
}
