//go:build !windows

package updater

import (
	"os/exec"
	"syscall"
)

func configureCoordinator(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
