package test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestUpgradeFlow_smokeChecksBinaryAndDaemonVersions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix smoke script")
	}
	state := t.TempDir()
	fakeCLI := `#!/usr/bin/env bash
set -euo pipefail
# solomon-cli-upgrade-v1
case "$1 ${2:-}" in
  'version --binary') cat "$FAKE_STATE_DIR/binary" ;;
  'version ') cat "$FAKE_STATE_DIR/daemon" ;;
  'server stop') echo 'daemon unavailable' > "$FAKE_STATE_DIR/daemon" ;;
  'upgrade ')
    [[ -t 0 ]]
    echo "$RELEASE_TAG" > "$FAKE_STATE_DIR/binary"
    echo "$RELEASE_TAG" > "$FAKE_STATE_DIR/daemon"
    ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(state, "solomon"), []byte(fakeCLI), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, version := range map[string]string{"binary": "v2026.1009.0", "daemon": "v2026.1009.1"} {
		if err := os.WriteFile(filepath.Join(state, name), []byte(version+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", "-c", `
set -euo pipefail
source "$1"
exe_path() { printf '%s/solomon' "$FAKE_STATE_DIR"; }
sleep() { command sleep 0.02; }
# A daemon at the target version cannot hide an outdated installed binary.
if wait_for_target_version "$(exe_path)" "" 1 > "$FAKE_STATE_DIR/stale.log" 2>&1; then
  echo 'accepted an outdated binary' >&2
  exit 1
fi
# Installation alone does not start a daemon. The upgrade must start and update it.
install_release() {
  echo "$1" > "$FAKE_STATE_DIR/binary"
  echo 'daemon unavailable' > "$FAKE_STATE_DIR/daemon"
}
run_case v2026.1009.0
`, "smoke-test", filepath.Join(repoRoot(t), "scripts", "check_upgrade_smoke.sh"))
	command.Env = append(os.Environ(), "FAKE_STATE_DIR="+state, "UPGRADE_SMOKE_ROOT="+state,
		"RELEASE_TAG=v2026.1009.1", "GITHUB_REPOSITORY=test/smoke")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("upgrade smoke version checks: %v\n%s", err, output)
	}
}
