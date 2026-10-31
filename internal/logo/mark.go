package logo

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func MarkSVG() string {
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="Solomon">`, iconSize, iconSize, iconSize, iconSize)
	for _, dot := range iconDots() {
		fmt.Fprintf(&svg, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="1.2" fill="%s"/>`, dot.x, dot.y, dot.width, dot.height, dot.fill)
	}
	svg.WriteString(`</svg>`)
	return svg.String()
}
func MarkPNG(size int) ([]byte, error) { return renderIconPNG(size, false) }
func Branding() (map[string]any, error) {
	png, err := MarkPNG(128)
	if err != nil {
		return nil, err
	}
	return map[string]any{"icons": []any{map[string]any{"src": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), "mimeType": "image/png", "sizes": []string{"128x128"}}}}, nil
}
