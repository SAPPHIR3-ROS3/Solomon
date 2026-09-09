//go:build darwin || linux

package test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_forceStopCleansViteProcessGroup(t *testing.T) {
	port := freeServerPortForTest(t)
	directory := t.TempDir()
	scriptPath := filepath.Join(directory, "vite-tree")
	script := "#!/bin/sh\n\"$SOLOMON_TEST_BINARY\" -test.run=TestServerViteHelperProcess -- vite \"$@\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(scriptPath, "run", "dev", "--", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	command.Env = append(os.Environ(),
		"SOLOMON_TEST_BINARY="+testBinaryForServer(t),
		"SOLOMON_TEST_VITE_HELPER=1",
	)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if stopped {
			return
		}
		serverruntime.ForceStop(serverruntime.State{VitePID: command.Process.Pid})
		_ = command.Wait()
	}()

	url := "http://127.0.0.1:" + strconv.Itoa(port)
	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			_ = response.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if response, err := client.Get(url); err != nil {
		t.Fatal("fake Vite process did not become ready")
	} else {
		_ = response.Body.Close()
	}

	serverruntime.ForceStop(serverruntime.State{VitePID: command.Process.Pid})
	stopped = true
	if err := command.Wait(); err == nil {
		t.Fatal("Vite process group exited without a termination error")
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err != nil {
			return
		}
		_ = response.Body.Close()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Vite child still responds at %s after force-stop", url)
}
