package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBubbleTeaControls(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(root, "control-tests-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	files, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find source files: %v", err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(path)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.CopyFS(filepath.Join(dir, "fixtures"), os.DirFS(filepath.Join(root, "fixtures"))); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("controls.fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "controls_test.go"), data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("model tests failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
