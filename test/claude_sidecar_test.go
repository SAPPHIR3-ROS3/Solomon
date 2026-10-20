package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/claude"
)

func TestClaudeSidecarSourcesResolveFromRepo(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "integrations", "claude", "src", "index.ts")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("repo root not found")
		}
		root = parent
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = claude.Ensure(ctx, root)
	if err == nil {
		t.Fatal("expected cancelled ensure to fail before serving")
	}
}
