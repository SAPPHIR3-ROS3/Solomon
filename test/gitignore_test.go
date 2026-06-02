package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/gitignore"
)

func TestGitignoreStack_ignoresPattern(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("skip.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := gitignore.NewStack(root)
	if !s.Ignored(filepath.Join(root, "skip.txt"), false) {
		t.Fatal("expected skip.txt ignored")
	}
	if s.Ignored(filepath.Join(root, "main.go"), false) {
		t.Fatal("expected main.go not ignored")
	}
}
