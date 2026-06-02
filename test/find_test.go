package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestFind_filesGlob(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]any{"pattern": "*.go", "files": true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := agenttools.Exec(ctx, &agenttools.Env{ProjRoot: dir}, "build", tooling.Invocation{Name: "find", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	matches, _ := m["matches"].([]string)
	if len(matches) != 1 || matches[0] != "a.go" {
		t.Fatalf("matches: %v", matches)
	}
}

func TestFind_textSearch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("func RegisterTool() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]any{"pattern": "RegisterTool", "files": false})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := agenttools.Exec(ctx, &agenttools.Env{ProjRoot: dir}, "build", tooling.Invocation{Name: "find", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	s, _ := m["output"].(string)
	if s == "" || !strings.Contains(s, "RegisterTool") {
		t.Fatalf("output: %q", s)
	}
}

func TestFind_respectsGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("skip.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("needle"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("needle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]any{"pattern": "needle", "files": false})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := agenttools.Exec(ctx, &agenttools.Env{ProjRoot: root}, "build", tooling.Invocation{Name: "find", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	s, _ := m["output"].(string)
	if strings.Contains(s, "skip.txt") {
		t.Fatalf("gitignored file in output: %q", s)
	}
	if !strings.Contains(s, "keep.go") {
		t.Fatalf("expected keep.go in output: %q", s)
	}
}
