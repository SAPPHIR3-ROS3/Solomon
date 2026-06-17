package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestListDir_immediateChildren(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]string{"path": "."})
	if err != nil {
		t.Fatal(err)
	}
	out, err := execDeferredToolForTest(&agenttools.Env{ProjRoot: dir}, tooling.Invocation{Name: "listDir", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["count"] != 2 {
		t.Fatalf("count=%v entries=%v", m["count"], m["entries"])
	}
	entries, _ := m["entries"].([]map[string]any)
	if len(entries) != 2 {
		t.Fatalf("entries: %v", m["entries"])
	}
	first := entries[0]
	if first["type"] != "dir" || first["name"] != "sub" {
		t.Fatalf("first entry: %v", first)
	}
}

func TestListDir_respectsGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("skip.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.go"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]string{"path": "."})
	if err != nil {
		t.Fatal(err)
	}
	out, err := execDeferredToolForTest(&agenttools.Env{ProjRoot: dir}, tooling.Invocation{Name: "listDir", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["count"] != 1 {
		t.Fatalf("count=%v entries=%v", m["count"], m["entries"])
	}
}

func TestTree_asciiStructure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]any{"path": ".", "maxDepth": 3})
	if err != nil {
		t.Fatal(err)
	}
	out, err := execDeferredToolForTest(&agenttools.Env{ProjRoot: dir}, tooling.Invocation{Name: "tree", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	tree, _ := m["tree"].(string)
	if !strings.Contains(tree, "pkg") || !strings.Contains(tree, "main.go") || !strings.Contains(tree, "README.md") {
		t.Fatalf("tree:\n%s", tree)
	}
	if !strings.Contains(tree, "├──") && !strings.Contains(tree, "└──") {
		t.Fatalf("expected branch chars: %q", tree)
	}
}

func TestListDir_rejectedInAgentWithoutDeferred(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]string{"path": "."})
	_, err := agenttools.Exec(t.Context(), &agenttools.Env{ProjRoot: dir}, "agent", tooling.Invocation{Name: "listDir", Args: args})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("want mode rejection, got %v", err)
	}
}
