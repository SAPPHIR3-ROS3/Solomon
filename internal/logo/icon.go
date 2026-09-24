//go:generate go run ../../scripts/generate_icon.go

package logo

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// IconBackground is the existing deep-sapphire GUI canvas color.
	IconBackground = "#061C3B"
	iconBorder     = "#174875"
	iconSize       = 512
	iconMarkScale  = 1.0 / 3.0
	iconCellWidth  = 16.0 * iconMarkScale
	iconCellHeight = 24.5 * iconMarkScale
	iconDotWidth   = 6.0 * iconMarkScale
	iconDotHeight  = 5.4 * iconMarkScale
	iconDotRadius  = 1.2 * iconMarkScale
)

type iconDot struct {
	x, y, width, height float64
	fill                string
}

type brailleDot struct {
	column int
	row    int
}

var brailleDotPositions = [...]brailleDot{
	{column: 0, row: 0},
	{column: 0, row: 1},
	{column: 0, row: 2},
	{column: 1, row: 0},
	{column: 1, row: 1},
	{column: 1, row: 2},
	{column: 0, row: 3},
	{column: 1, row: 3},
}

func iconDots() []iconDot {
	// The mark is authored with Unicode Braille cells; expand each cell's
	// eight-bit pattern and apply its matching color-map entry to every dot.
	artRows := strings.Split(strings.TrimRight(strings.ReplaceAll(ASCII, "\r\n", "\n"), "\n"), "\n")
	colorRows := strings.Split(strings.TrimRight(strings.ReplaceAll(ColorsRaw, "\r\n", "\n"), "\n"), "\n")
	columns := 0
	for _, row := range artRows {
		if width := utf8.RuneCountInString(row); width > columns {
			columns = width
		}
	}
	markWidth := float64(columns) * iconCellWidth
	markHeight := float64(len(artRows)) * iconCellHeight
	left := (iconSize - markWidth) / 2
	top := (iconSize - markHeight) / 2

	dots := make([]iconDot, 0, 260)
	for rowIndex, artRow := range artRows {
		if rowIndex >= len(colorRows) {
			break
		}
		colors := strings.Fields(colorRows[rowIndex])
		glyphs := []rune(artRow)
		for columnIndex, glyph := range glyphs {
			if glyph < 0x2800 || glyph > 0x28ff {
				continue
			}
			pattern := uint16(glyph - 0x2800)
			if pattern == 0 || columnIndex >= len(colors) || colors[columnIndex] == "000000" {
				continue
			}
			if len(colors[columnIndex]) != 6 {
				continue
			}
			if _, err := strconv.ParseUint(colors[columnIndex], 16, 24); err != nil {
				continue
			}
			fill := "#" + colors[columnIndex]
			for bit, position := range brailleDotPositions {
				if pattern&(1<<bit) == 0 {
					continue
				}
				cellX := left + float64(columnIndex)*iconCellWidth
				cellY := top + float64(rowIndex)*iconCellHeight
				dotCellWidth := iconCellWidth / 2
				dotCellHeight := iconCellHeight / 4
				dots = append(dots, iconDot{
					x:      cellX + float64(position.column)*dotCellWidth + (dotCellWidth-iconDotWidth)/2,
					y:      cellY + float64(position.row)*dotCellHeight + (dotCellHeight-iconDotHeight)/2,
					width:  iconDotWidth,
					height: iconDotHeight,
					fill:   fill,
				})
			}
		}
	}
	return dots
}

// IconSVG renders the terminal Braille banner as a rounded-square app mark.
func IconSVG() string {
	var svg strings.Builder
	svg.Grow(32000)
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="Solomon">`, iconSize, iconSize, iconSize, iconSize)
	svg.WriteString(`<rect width="512" height="512" rx="112" fill="` + IconBackground + `"/><rect x="3" y="3" width="506" height="506" rx="109" fill="none" stroke="` + iconBorder + `" stroke-width="6"/>`)
	for _, dot := range iconDots() {
		fmt.Fprintf(&svg, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" fill="%s"/>`, dot.x, dot.y, dot.width, dot.height, iconDotRadius, dot.fill)
	}
	svg.WriteString(`</svg>`)
	return svg.String()
}

// IconPNG returns a transparent, anti-aliased raster rendering of IconSVG.
func IconPNG(size int) ([]byte, error) {
	if size < 1 {
		return nil, fmt.Errorf("icon size must be positive")
	}
	const supersampling = 2
	const borderWidth = 3.0
	largeSize := size * supersampling
	outputScale := float64(size) / iconSize
	large := image.NewNRGBA(image.Rect(0, 0, largeSize, largeSize))
	background := parseIconColor(IconBackground)
	border := parseIconColor(iconBorder)

	for y := 0; y < largeSize; y++ {
		for x := 0; x < largeSize; x++ {
			px := (float64(x) + 0.5) / supersampling / outputScale
			py := (float64(y) + 0.5) / supersampling / outputScale
			if inRoundedRect(px, py, 0, 0, iconSize, iconSize, 112) {
				large.SetNRGBA(x, y, border)
				if inRoundedRect(px, py, borderWidth, borderWidth, iconSize-2*borderWidth, iconSize-2*borderWidth, 109) {
					large.SetNRGBA(x, y, background)
				}
			}
		}
	}

	for _, dot := range iconDots() {
		fill := parseIconColor(dot.fill)
		left := max(0, int(math.Floor(dot.x*outputScale*supersampling)))
		top := max(0, int(math.Floor(dot.y*outputScale*supersampling)))
		right := min(largeSize, int(math.Ceil((dot.x+dot.width)*outputScale*supersampling)))
		bottom := min(largeSize, int(math.Ceil((dot.y+dot.height)*outputScale*supersampling)))
		for y := top; y < bottom; y++ {
			py := (float64(y) + 0.5) / supersampling / outputScale
			if py < dot.y || py >= dot.y+dot.height {
				continue
			}
			for x := left; x < right; x++ {
				px := (float64(x) + 0.5) / supersampling / outputScale
				if inRoundedRect(px, py, dot.x, dot.y, dot.width, dot.height, iconDotRadius) {
					large.SetNRGBA(x, y, fill)
				}
			}
		}
	}

	icon := downsampleIcon(large, size, supersampling)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, icon); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// OAuthCallbackHTML returns the shared branded page shown after provider sign-in.
func OAuthCallbackHTML(title, message string, ok bool) string {
	iconData := base64.StdEncoding.EncodeToString([]byte(IconSVG()))
	statusColor := "#A83B3B"
	statusLabel := "Access needs attention"
	if ok {
		statusColor = "#237A52"
		statusLabel = "Access confirmed"
	}
	var page strings.Builder
	page.Grow(4096 + len(title) + len(message))
	page.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="` + IconBackground + `"><link rel="icon" type="image/svg+xml" href="data:image/svg+xml;base64,` + iconData + `"><title>`)
	page.WriteString(htmlEscape(title))
	page.WriteString(` · Solomon</title><style>
:root{color-scheme:dark;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#061C3B;color:#E8E5DF}*{box-sizing:border-box}body{min-height:100vh;margin:0;display:grid;place-items:center;padding:24px;background:radial-gradient(ellipse at 50% 34%,#0D3566 0%,#082A54 34%,#061C3B 76%)}main{width:min(100%,520px);padding:42px 40px 38px;border:1px solid #174875;border-radius:28px;background:#082A54;box-shadow:0 24px 80px #020b1b80;text-align:center}.brand{display:block;width:104px;height:104px;margin:0 auto 26px;border-radius:24px}.eyebrow{margin:0 0 12px;color:#86C9F2;font-size:12px;font-weight:700;letter-spacing:.16em;text-transform:uppercase}.status{display:inline-flex;align-items:center;gap:8px;margin-bottom:18px;color:` + statusColor + `;font-size:13px;font-weight:650}.status-dot{width:8px;height:8px;border-radius:50%;background:` + statusColor + `;box-shadow:0 0 14px ` + statusColor + `}h1{margin:0;color:#fff;font-size:clamp(25px,6vw,34px);font-weight:650;letter-spacing:-.035em;line-height:1.15}p{margin:16px 0 0;color:#C7D2D8;font-size:16px;line-height:1.65}.product{margin:30px 0 0;color:#74818A;font-size:12px;letter-spacing:.08em;text-transform:uppercase}@media(max-width:480px){main{padding:32px 24px;border-radius:22px}.brand{width:88px;height:88px;border-radius:20px}}
</style></head><body><main><img class="brand" src="data:image/svg+xml;base64,` + iconData + `" alt="Solomon"><p class="eyebrow">Solomon</p><div class="status"><span class="status-dot" aria-hidden="true"></span>` + statusLabel + `</div><h1>`)
	page.WriteString(htmlEscape(title))
	page.WriteString(`</h1><p>`)
	page.WriteString(htmlEscape(message))
	page.WriteString(`</p><p class="product">Secure provider sign-in</p></main></body></html>`)
	return page.String()
}

func parseIconColor(value string) color.NRGBA {
	if len(value) == 7 && value[0] == '#' {
		parsed, err := strconv.ParseUint(value[1:], 16, 32)
		if err == nil {
			return color.NRGBA{R: uint8(parsed >> 16), G: uint8(parsed >> 8), B: uint8(parsed), A: 255}
		}
	}
	return color.NRGBA{R: 255, G: 199, B: 4, A: 255}
}

func inRoundedRect(x, y, left, top, width, height, radius float64) bool {
	dx := math.Max(math.Abs(x-(left+width/2))-(width/2-radius), 0)
	dy := math.Max(math.Abs(y-(top+height/2))-(height/2-radius), 0)
	return dx*dx+dy*dy <= radius*radius
}

func downsampleIcon(large *image.NRGBA, size, scale int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	count := scale * scale
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var red, green, blue, alpha int
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					pixel := large.NRGBAAt(x*scale+sx, y*scale+sy)
					red += int(pixel.R) * int(pixel.A)
					green += int(pixel.G) * int(pixel.A)
					blue += int(pixel.B) * int(pixel.A)
					alpha += int(pixel.A)
				}
			}
			if alpha == 0 {
				continue
			}
			result.SetNRGBA(x, y, color.NRGBA{
				R: uint8((red + alpha/2) / alpha), G: uint8((green + alpha/2) / alpha),
				B: uint8((blue + alpha/2) / alpha), A: uint8((alpha + count/2) / count),
			})
		}
	}
	return result
}

func htmlEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(value)
}
