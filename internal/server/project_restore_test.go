package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRestoreGitPathsPreservesIndexAndOtherFiles(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init")
	write("selected.txt", "original")
	write("other.txt", "original")
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	write("selected.txt", "staged")
	git("add", "selected.txt")
	write("selected.txt", "unstaged")
	write("other.txt", "keep")
	status, err := restoreGitPaths(root, []string{"selected.txt"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"selected.txt": "staged", "other.txt": "keep"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
	if status.Staged["selected.txt"] != "M" || status.Changes["selected.txt"] != "" || status.Changes["other.txt"] != "M" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err := restoreGitPaths(root, []string{"../outside"}); err == nil {
		t.Fatal("accepted path outside repository")
	}
}
