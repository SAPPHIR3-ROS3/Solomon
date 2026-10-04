package updater

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
)

func configureCoordinator(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | windows.CREATE_BREAKAWAY_FROM_JOB}
}
