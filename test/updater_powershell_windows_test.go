//go:build windows

package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestWindowsPowerShellExePrefersPwsh(t *testing.T) {
	t.Parallel()
	exe := updater.WindowsPowerShellExeForTest()
	if !strings.EqualFold(filepath.Base(exe), "pwsh.exe") {
		t.Fatalf("expected pwsh.exe, got %q", exe)
	}
}
