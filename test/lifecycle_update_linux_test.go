package test

import (
	"context"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCoordinatedUpdateSelectsOnlyExactDesktopExecutable(t *testing.T) {
	// Use disposable native sleep binaries; never inspect or signal live clients.
	data, err := os.ReadFile("/bin/sleep")
	if err != nil {
		t.Fatal(err)
	}
	var commands []*exec.Cmd
	var paths []string
	for range 2 {
		path := filepath.Join(t.TempDir(), "solomon-desktop")
		if err := os.WriteFile(path, data, 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(path, "60")
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, command)
		paths = append(paths, path)
		defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	}
	processes, err := lifecycle.DesktopProcessesForTest(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 1 || processes[0].PID != commands[0].Process.Pid || processes[0].Identity == "" {
		t.Fatalf("selected unrelated desktop: %+v", processes)
	}
	identity := processes[0].Identity
	_ = commands[0].Process.Kill()
	_ = commands[0].Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lifecycle.WaitProcessExitForTest(ctx, commands[0].Process.Pid, identity); err != nil {
		t.Fatal(err)
	}
	if lifecycle.ProcessIdentityForTest(commands[1].Process.Pid) == "" {
		t.Fatal("unrelated process was interrupted")
	}
}
