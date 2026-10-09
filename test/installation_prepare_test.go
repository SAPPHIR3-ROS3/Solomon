package test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestSourceInstallPreparationFailureLeavesDaemonAndBinaryUntouched(t *testing.T) {
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	dir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	target := filepath.Join(dir, "solomon"+suffix)
	if err := os.WriteFile(target, []byte("original CLI"), 0755); err != nil {
		t.Fatal(err)
	}
	// A real failing executable avoids shell and .cmd dispatch differences.
	makeSource := filepath.Join(dir, "failed_make.go")
	if err := os.WriteFile(makeSource, []byte("package main\nimport \"os\"\nfunc main() { os.Exit(42) }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	failingMake := filepath.Join(dir, "make"+suffix)
	if output, err := exec.Command("go", "build", "-o", failingMake, makeSource).CombinedOutput(); err != nil {
		t.Fatalf("build failing compiler fixture: %v\n%s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "run", "./scripts/hot_install", target, failingMake, dir)
	command.Dir = repoRoot(t)
	if output, err := command.CombinedOutput(); err == nil || !strings.Contains(string(output), "running service preserved") {
		t.Fatalf("preparation failure: %v\n%s", err, output)
	}
	saved, err := serverruntime.LoadState()
	if err != nil || saved.PID != state.PID || !saved.StartedAt.Equal(state.StartedAt) {
		t.Fatalf("preparation changed daemon: %+v %v", saved, err)
	}
	health := getHealthForTest(t, state.URL)
	if !health.OK || health.Server.PID != state.PID {
		t.Fatal("daemon was interrupted before installation was ready")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "original CLI" {
		t.Fatalf("preparation overwrote CLI: %s %v", data, err)
	}
	staging, err := filepath.Glob(filepath.Join(dir, ".solomon-install-*"))
	if err != nil || len(staging) != 0 {
		t.Fatalf("preparation leaked staging: %v %v", staging, err)
	}
}
