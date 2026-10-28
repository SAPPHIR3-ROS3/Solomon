package test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
)

func TestReadDroppedImagesReturnsBytesForThreePaths(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	var payloads [][]byte
	for format, data := range readImageFixtures(t) {
		path := filepath.Join(dir, "Screenshot with spaces."+format)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		payloads = append(payloads, data)
	}
	images, err := clipboard.ReadDroppedImages(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 3 {
		t.Fatalf("image count = %d", len(images))
	}
	for i, image := range images {
		if image.Name != filepath.Base(paths[i]) || !strings.HasPrefix(image.Data, "data:image/") || !strings.HasSuffix(image.Data, ";base64,"+base64.StdEncoding.EncodeToString(payloads[i])) {
			t.Fatalf("image %d did not preserve name and bytes", i)
		}
	}
}

func TestReadDroppedImagesRejectsUnsupportedMissingAndOversizeFiles(t *testing.T) {
	dir := t.TempDir()
	text := filepath.Join(dir, "fake.png")
	if err := os.WriteFile(text, []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(dir, "large.png")
	if err := os.WriteFile(large, make([]byte, (5<<20)+1), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative.png", filepath.Join(dir, "missing.png"), text, large, dir} {
		if images, err := clipboard.ReadDroppedImages([]string{path}); err == nil || len(images) != 0 {
			t.Fatalf("accepted %s", path)
		}
	}
}
