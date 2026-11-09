//go:build !windows

package server

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"os/exec"
	"syscall"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
)

func configureManagedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopManagedProcess(cmd *exec.Cmd) {
	if stopSupervisedVite(cmd) {
		return
	}
	if cmd == nil || cmd.Process == nil {
		return
	}
	logging.ProcessTermination(cmd.Process.Pid, "server.managed", "managed_stop")
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_, _ = cmd.Process.Wait()
}

func ForceStopPID(pid int) {
	logging.ProcessTermination(pid, "server.force_stop", "force_stop_requested")
	if pid <= 1 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// ForceStop terminates the server and, when present, its separately managed
// Vite process group. Detached Solomon and Vite commands are process-group
// leaders, while a foreground "server run" may share its terminal's group;
// forceStopProcess handles both cases without killing an unrelated shell.
func ForceStop(state State) {
	if state.VitePID > 1 && state.VitePID != state.PID && state.ViteIdentity != "" && processidentity.Read(state.VitePID) == state.ViteIdentity {
		forceStopProcess(state.VitePID)
	}
	if state.PID > 1 && state.PIDIdentity != "" && processidentity.Read(state.PID) == state.PIDIdentity {
		forceStopProcess(state.PID)
	}
}

func forceStopProcess(pid int) {
	logging.ProcessTermination(pid, "server.force_stop", "force_stop_requested")
	if pid <= 1 {
		return
	}
	if processGroupID, err := syscall.Getpgid(pid); err == nil && processGroupID == pid {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func killManagedTree(pid int) {
	logging.ProcessTermination(pid, "server.managed_tree", "managed_tree_cleanup")
	if pid > 1 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}
