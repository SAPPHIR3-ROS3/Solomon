package test

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestUpgradeFlow_smokePTYProvidesControllingTerminal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix controlling terminal")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "upgrade_smoke_pty")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./scripts/upgrade_smoke_pty")
	build.Dir = repoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build PTY runner: %v\n%s", err, output)
	}
	command := exec.CommandContext(ctx, binary, "sh", "-c", `exec 3<>/dev/tty
test -t 0 && test -t 3 && test "$1" = 'argument with spaces' || exit 1
printf 'controlling terminal OK\n'
exit 17`, "probe", "argument with spaces")
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
		t.Fatalf("exit status not preserved: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "controlling terminal OK") {
		t.Fatalf("terminal verification failed: %s", output)
	}
}
