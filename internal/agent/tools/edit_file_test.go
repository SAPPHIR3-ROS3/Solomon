package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

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
	_, err = execEditFile(&Env{ProjRoot: dir, CheckpointStageProjAbs: func(string) {}}, args)
	if err == nil || !strings.Contains(err.Error(), "empty overwrite") {
		t.Fatalf("expected empty overwrite rejection, got %v", err)
	}
}
