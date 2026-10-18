//go:build !windows

package clipboard

import "os/exec"

func clipboardPowerShell(script string) *exec.Cmd {
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", script)
}
