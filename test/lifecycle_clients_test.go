package test

import (
	"context"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLifecycleClientHelper(t *testing.T) {
	if os.Getenv("SOLOMON_LIFECYCLE_HELPER") != "1" {
		return
	}
	stop, cleanup, err := lifecycle.RegisterClient(context.Background(), "desktop", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	select {
	case <-stop:
	case <-time.After(20 * time.Second):
		t.Fatal("no graceful stop request")
	}
}

func TestLifecycleRegisteredClientStopsGracefullyAndRejectsStaleIdentity(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("SOLOMON_HOME", home)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	name := "solomon-desktop"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable = filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(executable, data, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestLifecycleClientHelper$")
	command.Env = append(os.Environ(), "SOLOMON_LIFECYCLE_HELPER=1")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	directory := filepath.Join(home, "run", "clients")
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, _ := os.ReadDir(directory)
		ready := false
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") {
				ready = true
			}
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client did not register")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// A reused PID record must not get a shutdown request, even at the same path.
	stale := filepath.Join(directory, "stale.json")
	record := map[string]any{"pid": os.Getpid(), "identity": "stale", "executable": executable, "kind": "desktop"}
	staleData, _ := json.Marshal(record)
	if err := os.WriteFile(stale, staleData, 0600); err != nil {
		t.Fatal(err)
	}
	// The registered desktop may be outside the CLI's installation directory.
	clients, err := lifecycle.UpdateClientsForTest(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 1 || clients[0].PID != command.Process.Pid {
		t.Fatalf("wrong clients: %+v", clients)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale registration retained: %v", err)
	}
	if err := lifecycle.StopClientForTest(executable, command.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("client did not stop gracefully: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("registration cleanup: %v %v", entries, err)
	}
}
