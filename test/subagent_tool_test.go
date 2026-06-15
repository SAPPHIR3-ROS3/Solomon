package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestParseSubagentArgsDefaultSync(t *testing.T) {
	raw := json.RawMessage(`{"sysPromptPath":"agent.tmpl","task":"x"}`)
	a, err := agenttools.ParseSubagentArgsForTest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.RunInBackground {
		t.Fatal("default should be synchronous")
	}
}

func TestParseSubagentArgsRunInBackground(t *testing.T) {
	raw := json.RawMessage(`{"task":"x","run_in_background":true}`)
	a, err := agenttools.ParseSubagentArgsForTest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !a.RunInBackground {
		t.Fatal("run_in_background true")
	}
}

func TestParseSubagentArgsRunInBackgroundString(t *testing.T) {
	raw := json.RawMessage(`{"task":"x","run_in_background":"true"}`)
	a, err := agenttools.ParseSubagentArgsForTest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !a.RunInBackground {
		t.Fatal("string true should enable background")
	}
}

func TestFormatToolDisplayLines_subagentAsyncModeInHeader(t *testing.T) {
	args, _ := json.Marshal(map[string]any{
		"sysPromptPath":     "agent.tmpl",
		"task":              "Rispondi solamente OK",
		"run_in_background": true,
	})
	lines := tooling.FormatToolDisplayLines("subagent", args)
	if len(lines) < 2 {
		t.Fatalf("want at least 2 lines, got %d", len(lines))
	}
	plain0 := termcolor.Plain(lines[0])
	if plain0 != "Tool: subagent agent (async)" {
		t.Fatalf("header %q", plain0)
	}
	if len(lines) != 2 {
		t.Fatalf("async should not add extra lines, got %d: %v", len(lines), lines)
	}
}

func TestResolveSysPromptPath_defaultsToSolomonTemplates(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	got := agenttools.ResolveSysPromptPath(proj, "agent.tmpl")
	want := filepath.Join(home, "prompts", "templates", "agent.tmpl")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatal(err)
	}
}

func TestResolveSysPromptPath_projectFileFallback(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(proj, "sys.txt")
	if err := os.WriteFile(custom, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := agenttools.ResolveSysPromptPath(proj, "sys.txt")
	if got != custom {
		t.Fatalf("got %q want %q", got, custom)
	}
}

func TestResolveSysPromptPath_bareTemplateName(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	got := agenttools.ResolveSysPromptPath(proj, "agent")
	if !strings.HasSuffix(got, filepath.Join("prompts", "templates", "agent.tmpl")) {
		t.Fatalf("got %q", got)
	}
}
