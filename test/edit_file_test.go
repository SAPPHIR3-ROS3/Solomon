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

func TestExecEditFileDeleteRejectsOldOrNewString(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/keep.txt"
	if err := os.WriteFile(path, []byte("stay"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, argsMap := range map[string]map[string]any{
		"oldString": {
			"path": "keep.txt", "delete": true, "oldString": "stay", "intent": "bad delete",
		},
		"newString": {
			"path": "keep.txt", "delete": true, "newString": "gone", "intent": "bad delete",
		},
		"both": {
			"path": "keep.txt", "delete": true, "oldString": "stay", "newString": "gone", "intent": "bad delete",
		},
	} {
		args, err := json.Marshal(argsMap)
		if err != nil {
			t.Fatal(err)
		}
		_, err = execEditFileForTest(t, dir, args)
		if err == nil || !strings.Contains(err.Error(), "empty oldString and newString") {
			t.Fatalf("%s: expected delete string rejection, got %v", name, err)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
}

func TestExecEditFileRename(t *testing.T) {
	dir := t.TempDir()
	src := dir + "/old.txt"
	if err := os.WriteFile(src, []byte("move me"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]any{
		"path":     "old.txt",
		"renameTo": "new.txt",
		"intent":   "rename test file",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := execEditFileForTest(t, dir, args)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["action"] != "renamed" || m["ok"] != true {
		t.Fatalf("unexpected result: %#v", res)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("src should be gone: %v", err)
	}
	b, err := os.ReadFile(dir + "/new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "move me" {
		t.Fatalf("got %q", b)
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
