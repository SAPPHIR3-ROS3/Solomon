//go:build !windows

package server

import (
	"os/exec"
	"syscall"
)

func configureManagedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopManagedProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_, _ = cmd.Process.Wait()
}

func ForceStopPID(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// ForceStop terminates the server and, when present, its separately managed
// Vite process group. Detached Solomon and Vite commands are process-group
// leaders, while a foreground "server run" may share its terminal's group;
// forceStopProcess handles both cases without killing an unrelated shell.
func ForceStop(state State) {
	if state.VitePID > 1 && state.VitePID != state.PID {
		forceStopProcess(state.VitePID)
	}
	if state.PID > 1 {
		forceStopProcess(state.PID)
	}
}

func forceStopProcess(pid int) {
	if pid <= 1 {
		return
	}
	if processGroupID, err := syscall.Getpgid(pid); err == nil && processGroupID == pid {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
