//go:build windows

package server

import (
	"os/exec"
	"strconv"
	"syscall"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
)

const detachedProcess = 0x00000008
const createNoWindow = 0x08000000

func configureManagedProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

func runTaskkill(args ...string) {
	cmd := exec.Command("taskkill", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	_ = cmd.Run()
}

func stopManagedProcess(cmd *exec.Cmd) {
	if stopSupervisedVite(cmd) {
		return
	}
	if cmd == nil || cmd.Process == nil {
		return
	}
	runTaskkill("/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	_, _ = cmd.Process.Wait()
}

func ForceStopPID(pid int) {
	if pid <= 1 {
		return
	}
	runTaskkill("/PID", strconv.Itoa(pid), "/T", "/F")
}

// ForceStop terminates the detached server and, when present, its separately
// managed Vite process tree. Windows has no Unix-style process groups here;
// taskkill /T provides the equivalent tree cleanup.
func ForceStop(state State) {
	if state.VitePID > 1 && state.VitePID != state.PID && state.ViteIdentity != "" && processidentity.Read(state.VitePID) == state.ViteIdentity {
		runTaskkill("/PID", strconv.Itoa(state.VitePID), "/T", "/F")
	}
	if state.PID > 1 && state.PIDIdentity != "" && processidentity.Read(state.PID) == state.PIDIdentity {
		runTaskkill("/PID", strconv.Itoa(state.PID), "/T", "/F")
	}
}

func killManagedTree(pid int) {
	if pid > 1 {
		runTaskkill("/PID", strconv.Itoa(pid), "/T", "/F")
	}
}
