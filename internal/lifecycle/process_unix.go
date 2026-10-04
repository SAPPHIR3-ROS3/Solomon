//go:build !windows

package lifecycle

import (
	"os/exec"
	"syscall"
)

func configureClientProcess(command *exec.Cmd, kind string) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
func stopLegacyDesktop(pid int) error {
	err := syscall.Kill(pid, syscall.SIGTERM)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

func startClientProcess(command *exec.Cmd, kind string) error { return command.Start() }
