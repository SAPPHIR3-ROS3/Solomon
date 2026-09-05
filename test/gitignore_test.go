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

func TestGitignoreStack_nestedGitignoreDirDoesNotPanic(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("root-skip.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, ".gitignore"), []byte("inner.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "inner.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "keep.go"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := gitignore.NewStack(root)
	s.PushDir(nested)
	if s.Ignored(nested, true) {
		t.Fatal("expected nested dir itself not ignored")
	}
	if !s.Ignored(filepath.Join(nested, "inner.txt"), false) {
		t.Fatal("expected inner.txt ignored")
	}
	if s.Ignored(filepath.Join(nested, "keep.go"), false) {
		t.Fatal("expected keep.go not ignored")
	}
}
