package test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestDesktopLinuxRegistersExistingLocalGUI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "desktop data"))
	cli := filepath.Join(dir, "custom CLI directory", "solomon")
	if err := os.MkdirAll(filepath.Dir(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cli, []byte("CLI"), 0o755); err != nil {
		t.Fatal(err)
	}
	target, err := updater.DesktopExecutablePath(cli)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("GUI"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := updater.EnsureDesktop(context.Background(), "dev", cli, io.Discard); err != nil {
		t.Fatal(err)
	}
	entry, err := os.ReadFile(filepath.Join(os.Getenv("XDG_DATA_HOME"), "applications", "solomon.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entry), `Exec="`+target+`"`) || !strings.Contains(string(entry), "Terminal=false") {
		t.Fatalf("incorrect application entry: %s", entry)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_DATA_HOME"), "icons", "hicolor", "512x512", "apps", "solomon.png")); err != nil {
		t.Fatal("application icon missing")
	}
}
