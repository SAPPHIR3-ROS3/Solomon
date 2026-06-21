//go:build windows

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func windowsPowerShellExe() string {
	if p := findPwshExe(); p != "" {
		return p
	}
	if p, err := exec.LookPath("powershell.exe"); err == nil && p != "" {
		return p
	}
	systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRoot == "" {
		systemRoot = strings.TrimSpace(os.Getenv("windir"))
	}
	if systemRoot != "" {
		p := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "powershell"
}

func findPwshExe() string {
	for _, name := range []string{"pwsh.exe", "pwsh"} {
		if p, err := exec.LookPath(name); err == nil && p != "" {
			return filepath.Clean(p)
		}
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		p := filepath.Join(base, "PowerShell", "7", "pwsh.exe")
		if _, err := os.Stat(p); err == nil {
			return filepath.Clean(p)
		}
	}
	return ""
}
