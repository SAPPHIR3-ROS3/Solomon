package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/shellhist"
)

func TestShellhistParseZshExtended(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zsh_history")
	if err := os.WriteFile(path, []byte(": 1700000000:0;go test ./...\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTFILE", path)
	t.Setenv("SHELL", "/bin/zsh")
	if got := shellhist.Suggest("go t"); got != "go test ./..." {
		t.Fatalf("suggest got %q", got)
	}
}

func TestShellhistAppendPlain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	t.Setenv("HISTFILE", path)
	t.Setenv("SHELL", "/bin/bash")
	if err := shellhist.Append("echo hi"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "echo hi\n" {
		t.Fatalf("append got %q", string(b))
	}
}
