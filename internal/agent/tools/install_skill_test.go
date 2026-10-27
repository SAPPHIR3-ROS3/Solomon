package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestInstallSkillRegisteredAndGated(t *testing.T) {
	params, err := NativeToolParams("agent")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(params)
	if !strings.Contains(string(b), `"installSkill"`) {
		t.Fatal("missing native tool")
	}
	dump, err := BuildAgentToolDump()
	if err != nil || !strings.Contains(dump, "installSkill") {
		t.Fatalf("missing tool dump: %v", err)
	}
	if !modeAllowed(&Env{}, "agent", "installSkill") || modeAllowed(&Env{}, "chat", "installSkill") {
		t.Fatal("incorrect mode gating")
	}
}

func TestInstallSkillLocalMarkdown(t *testing.T) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	md := filepath.Join(root, "example.md")
	if err := os.WriteFile(md, []byte("---\nname: example\ndescription: Test skill\n---\nTest body\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"args": []string{"skill", md, "local"}, "intent": "Install requested skill"})
	result, err := Exec(context.Background(), &Env{ProjRoot: root, ProjHex: "abc"}, "agent", tooling.Invocation{Name: "installSkill", Args: raw})
	if err != nil {
		t.Fatal(err)
	}
	if result.(map[string]any)["ok"] != true {
		t.Fatalf("%v", result)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".solomon", "skills", "*", "SKILL.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("skill not installed: %v %v", matches, err)
	}
}
