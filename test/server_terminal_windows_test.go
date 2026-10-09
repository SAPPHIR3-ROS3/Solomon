//go:build windows

package test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestWindowsTerminalRedirectedDaemon(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	command := exec.Command(executable, "-test.run=^TestWindowsTerminalHelper$", "--", "parent")
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			data, _ := os.ReadFile(logPath)
			t.Fatalf("redirected daemon: %v\n%s", err, data)
		}
	case <-time.After(20 * time.Second):
		_ = command.Process.Kill()
		<-done
		t.Fatal("terminal output did not arrive from redirected daemon")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "terminal-stdout") || strings.Contains(string(data), "terminal-stderr") {
		t.Fatalf("terminal output leaked into daemon log: %s", data)
	}
}

func TestWindowsTerminalHelper(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "--" {
		return
	}
	if os.Args[len(os.Args)-1] == "child" {
		fmt.Fprintln(os.Stdout, "terminal-stdout")
		fmt.Fprintln(os.Stderr, "terminal-stderr")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil || strings.TrimSpace(line) != "terminal-input" {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "terminal-input-ok")
		os.Exit(0)
	}
	executable, _ := os.Executable()
	cwd, _ := os.Getwd()
	process, err := serverruntime.StartTerminalProcessForTest(executable,
		[]string{"-test.run=^TestWindowsTerminalHelper$", "--", "child"}, cwd)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()
	defer process.Kill()
	if _, err := process.Write([]byte("terminal-input\r\n")); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(process)
	if err != nil && !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"terminal-stdout", "terminal-stderr", "terminal-input-ok"} {
		if !strings.Contains(string(data), marker) {
			t.Fatalf("missing %s in pseudoconsole output: %q", marker, data)
		}
	}
}
