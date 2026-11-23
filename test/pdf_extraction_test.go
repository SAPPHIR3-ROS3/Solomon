package test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestExtractPDFImages(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	writePDFObjects(t, filepath.Join(dir, "fixture.pdf"), []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 2 /Kids [3 0 R 4 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 300] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 300] /Resources << /XObject << /Im 5 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /XObject /Subtype /Image /Width 2 /Height 1 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 6 >>\nstream\n\xff\x00\x00\x00\xff\x00\nendstream",
		"<< /Length 18 >>\nstream\nq /Im Do Q       \nendstream",
	})
	env := &tools.Env{ProjRoot: dir}
	extract := func(args string) (any, error) {
		return execDeferredToolForTest(env, tooling.Invocation{Name: "extractPDFImages", Args: json.RawMessage(args)})
	}
	out, err := extract(`{"path":"fixture.pdf","outputDir":"empty","firstPage":1,"lastPage":1,"intent":"Extract images from page one"}`)
	if err != nil || out.(map[string]any)["count"] != 0 {
		t.Fatalf("expected no images on page one: %v, %v", out, err)
	}
	const request = `{"path":"fixture.pdf","outputDir":"images","firstPage":2,"lastPage":2,"intent":"Extract images from page two"}`
	out, err = extract(request)
	if err != nil {
		t.Fatal(err)
	}
	images := out.(map[string]any)["images"].([]pdfinspect.ExtractedImage)
	if len(images) != 1 || images[0].Page != 2 {
		t.Fatalf("unexpected extracted images: %+v", images)
	}
	file, err := os.Open(images[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	cfg, err := png.DecodeConfig(file)
	if err != nil || cfg.Width != 2 || cfg.Height != 1 {
		t.Fatalf("expected embedded image dimensions, got %+v, %v", cfg, err)
	}
	if _, err := extract(request); err == nil {
		t.Fatal("expected existing destination to be rejected")
	}
}
