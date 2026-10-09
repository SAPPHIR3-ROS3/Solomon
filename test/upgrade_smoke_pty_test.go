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
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("upgrade smoke requires python3:", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	script := filepath.Join(repoRoot(t), "scripts", "upgrade_smoke_pty.py")
	command := exec.CommandContext(ctx, python, script, python, "-c", `
import os, sys
assert os.isatty(0)
with open('/dev/tty') as terminal:
    assert os.isatty(terminal.fileno())
assert sys.argv[1] == 'argument with spaces'
print('controlling terminal OK', flush=True)
sys.exit(17)
`, "argument with spaces")
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
		t.Fatalf("exit status not preserved: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "controlling terminal OK") {
		t.Fatalf("terminal verification failed: %s", output)
	}
}
