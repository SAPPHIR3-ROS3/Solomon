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

// ForceStop terminates the detached server and, when present, its separately
// managed Vite process tree. Windows has no Unix-style process groups here;
// taskkill /T provides the equivalent tree cleanup.
func ForceStop(state State) {
	if state.VitePID > 0 && state.VitePID != state.PID {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(state.VitePID), "/T", "/F").Run()
	}
	if state.PID > 0 {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(state.PID), "/T", "/F").Run()
	}
}
