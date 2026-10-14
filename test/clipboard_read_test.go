package test

import (
	"context"
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestReadLinuxImage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux clipboard commands")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n--list-types) printf 'text/plain\\nimage/jpeg\\n';;\n--no-newline) [ \"$2\" = --type ] && [ \"$3\" = image/jpeg ] || exit 1; printf 'image-bytes';;\n*) exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-paste"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	data, err := clipboard.ReadImage(context.Background())
	if err != nil || string(data) != "image-bytes" {
		t.Fatalf("clipboard.ReadImage = %q, %v", data, err)
	}
}

func TestReadLinuxClipboardWithoutImage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux clipboard commands")
	}
	t.Setenv("PATH", t.TempDir())
	_, err := clipboard.ReadImage(context.Background())
	if !errors.Is(err, clipboard.ErrNoImage) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadLinuxClipboardTimeout(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux clipboard commands")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wl-paste"), []byte("#!/bin/sh\nwhile :; do :; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := clipboard.ReadImage(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
}
