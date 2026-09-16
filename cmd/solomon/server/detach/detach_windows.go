//go:build windows

package detach

import (
	"os/exec"
	"syscall"
)

const detachedProcess = 0x00000008
const createNoWindow = 0x08000000

func Configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess | createNoWindow,
	}
}
