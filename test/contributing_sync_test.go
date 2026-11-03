package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContributingGuideMatchesRootCopy(t *testing.T) {
	root := repoRoot(t)
	source, err := os.ReadFile(filepath.Join(root, "docs", "development", "contributing.md"))
	if err != nil {
		t.Fatal(err)
	}
	copy, err := os.ReadFile(filepath.Join(root, "CONTRIBUTING.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != string(copy) {
		t.Fatal("CONTRIBUTING.md and docs/development/contributing.md differ; keep both copies identical")
	}
}
