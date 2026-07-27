//go:build windows

package server

import (
	"os/exec"
	"strconv"
)

func configureManagedProcess(_ *exec.Cmd) {}

func stopManagedProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
	_, _ = cmd.Process.Wait()
}

func ForceStopPID(pid int) {
	if pid <= 0 {
		return
	}
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
