package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestGoTestsLiveInTopLevelTestDirectory(t *testing.T) {
	root := repositoryRootForTestLayout(t)

	cmd := exec.Command("go", "list", "-json", "-test", "./...")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("go list -test ./... failed: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("run go list -test ./...: %v", err)
	}

	var misplaced []string
	seen := make(map[string]struct{})
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var pkg struct {
			Dir          string
			TestGoFiles  []string
			XTestGoFiles []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode go list -test output: %v", err)
		}

		for _, filename := range append(pkg.TestGoFiles, pkg.XTestGoFiles...) {
			path := filepath.Join(pkg.Dir, filename)
			rel, err := filepath.Rel(root, path)
			if err != nil {
				t.Fatalf("make test path relative to repository: %v", err)
			}
			if rel == "test" || strings.HasPrefix(rel, "test"+string(filepath.Separator)) {
				continue
			}
			rel = filepath.ToSlash(rel)
			if _, ok := seen[rel]; !ok {
				seen[rel] = struct{}{}
				misplaced = append(misplaced, rel)
			}
		}
	}

	sort.Strings(misplaced)
	if len(misplaced) > 0 {
		t.Fatalf("Go tests must live under test/; found outside test/:\n\t%s", strings.Join(misplaced, "\n\t"))
	}
}

func repositoryRootForTestLayout(t *testing.T) string {
	t.Helper()

	workingDir, workingDirErr := os.Getwd()
	if workingDirErr == nil {
		if root, ok := findRepositoryRootForTestLayout(workingDir); ok {
			return root
		}
	}

	if _, filename, _, ok := runtime.Caller(0); ok && filepath.IsAbs(filename) {
		if root, ok := findRepositoryRootForTestLayout(filepath.Dir(filename)); ok {
			return root
		}
	}

	if workingDirErr != nil {
		t.Fatalf("locate repository root: get working directory: %v", workingDirErr)
	}
	t.Fatalf("locate repository root from %s", workingDir)
	return ""
}

func findRepositoryRootForTestLayout(start string) (string, bool) {
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		goModInfo, goModErr := os.Stat(filepath.Join(dir, "go.mod"))
		testDirInfo, testDirErr := os.Stat(filepath.Join(dir, "test"))
		if goModErr == nil && !goModInfo.IsDir() && testDirErr == nil && testDirInfo.IsDir() {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
	}
}
