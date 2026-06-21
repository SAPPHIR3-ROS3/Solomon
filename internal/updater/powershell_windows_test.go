//go:build windows

package updater

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsPowerShellExePrefersPwsh(t *testing.T) {
	t.Parallel()
	exe := windowsPowerShellExe()
	if !strings.EqualFold(filepath.Base(exe), "pwsh.exe") {
		t.Fatalf("expected pwsh.exe, got %q", exe)
	}
}
