package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestReadFilePaginationRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	content := "alpha\nbeta\ngamma\ndelta\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	env := &agenttools.Env{ProjRoot: dir, ProjHex: testProjectHex}
	args, err := json.Marshal(map[string]any{"path": "sample.txt", "startLine": 2, "endLine": 3})
	if err != nil {
		t.Fatal(err)
	}
	res, err := agenttools.Exec(context.Background(), env, "build", tooling.Invocation{Name: "readFile", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", res)
	}
	if m["content"] != "beta\ngamma" {
		t.Fatalf("content=%q", m["content"])
	}
	if m["total_lines"] != 4 {
		t.Fatalf("total_lines=%v", m["total_lines"])
	}
	if m["start_line"] != 2 || m["end_line"] != 3 {
		t.Fatalf("range metadata wrong: %+v", m)
	}
}

func TestReadFilePaginationStartOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := &agenttools.Env{ProjRoot: dir, ProjHex: testProjectHex}
	start := 2
	args, _ := json.Marshal(map[string]any{"path": "f.txt", "startLine": start})
	res, err := agenttools.Exec(context.Background(), env, "build", tooling.Invocation{Name: "readFile", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["content"] != "two\nthree" {
		t.Fatalf("content=%q", m["content"])
	}
}

func TestReadFilePaginationErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := &agenttools.Env{ProjRoot: dir, ProjHex: testProjectHex}
	args, _ := json.Marshal(map[string]any{"path": "f.txt", "startLine": 5})
	_, err := agenttools.Exec(context.Background(), env, "build", tooling.Invocation{Name: "readFile", Args: args})
	if err == nil {
		t.Fatal("expected error for startLine beyond file")
	}
}

func TestReadFileFullFileMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := &agenttools.Env{ProjRoot: dir, ProjHex: testProjectHex}
	args, _ := json.Marshal(map[string]any{"path": "f.txt"})
	res, err := agenttools.Exec(context.Background(), env, "build", tooling.Invocation{Name: "readFile", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["total_lines"] != 1 {
		t.Fatalf("total_lines=%v", m["total_lines"])
	}
	if _, ok := m["start_line"]; ok {
		t.Fatal("full read should not set start_line")
	}
}
