package test

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
)

func TestACPBrandingPreservesCrownWithoutBackground(t *testing.T) {
	svg := logo.MarkSVG()
	if strings.Contains(svg, logo.IconBackground) || strings.Contains(svg, `stroke=`) {
		t.Fatal("transparent mark contains the app background or border")
	}
	branding, err := logo.Branding()
	if err != nil {
		t.Fatal(err)
	}
	descriptor := branding["icons"].([]any)[0].(map[string]any)
	src := descriptor["src"].(string)
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(src, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
		t.Fatal("unexpected icon dimensions")
	}
	for _, point := range [][2]int{{0, 0}, {64, 10}, {10, 64}, {64, 120}} {
		_, _, _, alpha := img.At(point[0], point[1]).RGBA()
		if alpha != 0 {
			t.Fatalf("background pixel is opaque at %v", point)
		}
	}
	visible := 0
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				visible++
			}
		}
	}
	if visible == 0 {
		t.Fatal("crown was removed with its background")
	}
}
func TestACPAdvertisesBrandingInExtensionMetadata(t *testing.T) {
	c := startACP(t, "", savedFactory)
	result := c.call(t, "initialize", map[string]any{"protocolVersion": 1})
	info := result["agentInfo"].(map[string]any)
	if info["title"] != "Solomon" {
		t.Fatal("missing display title")
	}
	meta := info["_meta"].(map[string]any)
	if meta["solomon/branding"] == nil {
		t.Fatal("missing branding metadata")
	}
}
