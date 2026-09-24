//go:build windows

package cursor

import (
	"os/exec"
	"syscall"
)

const detachedProcess = 0x00000008
const createNoWindow = 0x08000000

func configureSidecarProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
