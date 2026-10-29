package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageCLIStandaloneAndPoster(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "poster.png")
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 4, 4))
	im.Set(1, 1, color.White)
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	err := runImageCLI(context.Background(), []string{"--width", "4", "--color", "truecolor", "--preview", path, filepath.Join(dir, "not-decoded.mp4")}, &out, &errs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "38;2;") || !strings.HasSuffix(out.String(), "not-decoded.mp4\n") {
		t.Fatal(out.String())
	}
	for _, args := range [][]string{{"--mode", "bad", path}, {"--width", "-1", path}, {"--color", "bad", path}, {"--background", "gggggg", path}, {}} {
		if err := runImageCLI(context.Background(), args, &out, &errs); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
