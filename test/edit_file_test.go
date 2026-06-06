package test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func execEditFileForTest(t *testing.T, dir string, args json.RawMessage) (any, error) {
	t.Helper()
	return tools.Exec(context.Background(), &tools.Env{ProjRoot: dir, CheckpointStageProjAbs: func(string) {}}, "build", tooling.Invocation{Name: "editFile", Args: args})
}

func TestExecEditFileRejectsEmptyOverwrite(t *testing.T) {
	dir := t.TempDir()
	args, err := json.Marshal(map[string]string{
		"path":      "PLAN.md",
		"oldString": "",
		"newString": "",
		"intent":    "test empty overwrite",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = execEditFileForTest(t, dir, args)
	if err == nil || !strings.Contains(err.Error(), "empty overwrite") {
		t.Fatalf("expected empty overwrite rejection, got %v", err)
	}
}

func TestExecEditFileDelete(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/remove-me.txt"
	if err := os.WriteFile(path, []byte("gone"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]any{
		"path":   "remove-me.txt",
		"delete": true,
		"intent": "remove temp file",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := execEditFileForTest(t, dir, args)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["action"] != "deleted" || m["ok"] != true {
		t.Fatalf("unexpected result: %#v", res)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err=%v", err)
	}
}

func TestExecEditFileDeleteMissingFile(t *testing.T) {
	dir := t.TempDir()
	args, err := json.Marshal(map[string]any{
		"path":   "missing.txt",
		"delete": true,
		"intent": "remove missing file",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := execEditFileForTest(t, dir, args)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["ok"] != false || m["reason"] != "file not found" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestExecEditFileDeleteRequiresIntent(t *testing.T) {
	dir := t.TempDir()
	args, err := json.Marshal(map[string]any{
		"path":   "x.txt",
		"delete": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = execEditFileForTest(t, dir, args)
	if err == nil || !strings.Contains(err.Error(), "intent") {
		t.Fatalf("expected intent error, got %v", err)
	}
}

func TestBuildBuildToolDumpMentionsEditFileDelete(t *testing.T) {
	dump, err := tools.BuildBuildToolDump()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump, "delete=true") {
		t.Fatalf("expected delete=true in tool dump, got: %s", dump)
	}
}
