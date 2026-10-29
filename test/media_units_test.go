package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaInternalUnits(t *testing.T) {
	root := repositoryRootForTestLayout(t)
	replacements := map[string]string{}
	packages := map[string]bool{}
	fixtures := filepath.Join(root, "test", "mediaunits")
	err := filepath.WalkDir(fixtures, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go.txt") {
			return nil
		}
		rel, err := filepath.Rel(fixtures, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, strings.TrimSuffix(rel, ".txt"))
		replacements[target] = path
		packages["./"+filepath.ToSlash(filepath.Dir(rel))] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"Replace": replacements})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"test", "-overlay", overlay, "-count=1"}
	for pkg := range packages {
		args = append(args, pkg)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("media unit tests: %v\n%s", err, output)
	}
}
