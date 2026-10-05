package main

import (
	"os/exec"
	"syscall"
)

func configureDesktopProcess(cmd *exec.Cmd) {
	// Keep the GUI independent of the CLI without hiding its native window.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
