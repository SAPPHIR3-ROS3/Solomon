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
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	var commands []*exec.Cmd
	var paths []string
	for range 2 {
		directory, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, "solomon-desktop")
		if err := os.WriteFile(path, data, 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(path, "-test.run=^TestDesktopProcessHelper$")
		command.Env = append(os.Environ(), "SOLOMON_DESKTOP_PROCESS_HELPER=1")
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, command)
		paths = append(paths, path)
		defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	}
	processes, err := lifecycle.DesktopProcessesForTest(paths[0])
	deadline := time.Now().Add(5 * time.Second)
	for err == nil && len(processes) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		processes, err = lifecycle.DesktopProcessesForTest(paths[0])
	}
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

func TestDesktopProcessHelper(t *testing.T) {
	if os.Getenv("SOLOMON_DESKTOP_PROCESS_HELPER") != "1" {
		return
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}
