package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestSnapshotSourceTreePreReleaseInputs(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() string {
		t.Helper()
		tree, err := updater.SnapshotSourceTree(root)
		if err != nil {
			t.Fatal(err)
		}
		return tree
	}
	git("init")
	git("config", "core.autocrlf", "false")
	write(".gitignore", "release-output\n")
	write("source.go", "package example\n")
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "source before release")
	baseline := git("rev-parse", "HEAD^{tree}")
	if got := snapshot(); got != baseline {
		t.Fatalf("clean source = %s, want %s", got, baseline)
	}
	write("release-output", "binary containing version and checker metadata")
	if got := snapshot(); got != baseline {
		t.Fatal("generated release output changed source identity")
	}
	write("source.go", "package changed\n")
	git("add", "source.go")
	staged := git("ls-files", "--stage")
	write("source.go", "package unstaged\n")
	if got := snapshot(); got == baseline || got == git("write-tree") {
		t.Fatal("snapshot missed unstaged source edits")
	}
	if got := git("ls-files", "--stage"); got != staged {
		t.Fatal("snapshot modified the user's staging index")
	}
	write("source.go", "package example\n")
	if got := snapshot(); got != baseline {
		t.Fatal("snapshot followed stale staging instead of current source")
	}
	write("new.go", "package example\n")
	if got := snapshot(); got == baseline {
		t.Fatal("snapshot missed a new source file")
	}
	if err := os.Remove(filepath.Join(root, "new.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "source.go")); err != nil {
		t.Fatal(err)
	}
	if got := snapshot(); got == baseline {
		t.Fatal("snapshot missed source deletion")
	}
}
