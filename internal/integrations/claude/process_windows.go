//go:build windows

package claude

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}

func killProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
