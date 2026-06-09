package test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestRunSystemInstallSchedulesRestart(t *testing.T) {
	t.Parallel()
	restore := updater.SetScheduleInstallRestartHook(func(context.Context, string, io.Writer) error {
		return nil
	})
	defer restore()
	err := updater.RunSystemInstall(context.Background(), "v2099.1.0", io.Discard)
	if !errors.Is(err, updater.ErrRestartScheduled) {
		t.Fatalf("got %v", err)
	}
}

func TestUnixRestartScriptEmptyArgs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "restart.sh")
	body := `#!/usr/bin/env bash
set -euo pipefail
RESTART_EXE=/bin/echo
RESTART_ARGS=()
if ((${#RESTART_ARGS[@]} > 0)); then
  exec "$RESTART_EXE" "${RESTART_ARGS[@]}"
else
  exec "$RESTART_EXE" "restarted"
fi
`
	if err := os.WriteFile(scriptPath, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(scriptPath).CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	if string(out) != "restarted\n" {
		t.Fatalf("got %q", out)
	}
}
