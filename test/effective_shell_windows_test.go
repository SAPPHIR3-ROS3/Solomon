//go:build windows

package test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt/shell"
)

func TestEffectiveShell_notCmdWhenPSModulePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip()
	}
	t.Setenv("COMSPEC", `C:\Windows\System32\cmd.exe`)
	t.Setenv("PSModulePath", `C:\Program Files\WindowsPowerShell\Modules`)
	t.Setenv("PWSH_VERSION", "")
	got := shell.Effective()
	if strings.EqualFold(filepath.Base(got), "cmd.exe") {
		t.Fatalf("Effective shell: got cmd.exe with PSModulePath set: %q", got)
	}
}
