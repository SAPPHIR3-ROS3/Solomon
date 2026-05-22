package test

import (
	"encoding/json"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooloutput"
)

func TestRuntimeToolOutputWiringTruncatesShellResult(t *testing.T) {
	t.Cleanup(func() { _ = tooloutput.CleanupProjectTemp(testProjectHex) })
	p := &config.Provider{Name: "p", BaseURL: "http://127.0.0.1:9", APIKey: "k"}
	cfg := &config.Root{
		Current:   config.Current{Provider: "p", Model: "m"},
		Providers: map[string]*config.Provider{"p": p},
		ToolOutput: config.ToolOutput{
			MaxBytes: 300,
			MaxLines: 20,
		},
	}
	rt := agentruntime.NewRuntime(nil, cfg, p, testProjectHex, t.TempDir(), &chatstore.Session{ID: "integ-session"})
	if rt.ToolOut == nil {
		t.Fatal("ToolOut not initialized on Runtime")
	}
	lines := make([]string, 80)
	for i := range lines {
		lines[i] = strings.Repeat("o", 30)
	}
	in := map[string]any{
		"exit":   0,
		"output": strings.Join(lines, "\n"),
		"intent": "test",
	}
	out := rt.ToolOut.Apply(in, tooloutput.Meta{
		SessionID:  "integ-session",
		ToolCallID: "tool-call-1",
		ToolName:   "shell",
	})
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	if m["truncated"] != true {
		t.Fatal("expected truncation")
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(string(b)) > 2000 {
		t.Fatalf("persisted payload still too large: %d bytes", len(b))
	}
	if m["spill_path"] == nil || m["spill_path"] == "" {
		t.Fatal("expected spill_path in truncated result")
	}
}
