package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooloutput"
)

const testProjectHex = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func testToolOutputService(lim tooloutput.Limits) *tooloutput.Service {
	return tooloutput.NewService(testProjectHex, lim)
}

func TestToolOutputNoTruncateUnderLimits(t *testing.T) {
	svc := testToolOutputService(tooloutput.Limits{MaxBytes: 1024, MaxLines: 100})
	in := map[string]any{"output": "hello", "exit": 0}
	out := svc.Apply(in, tooloutput.Meta{SessionID: "s1", ToolCallID: "tc1", ToolName: "shell"})
	if out == nil {
		t.Fatal("nil result")
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	bIn, _ := json.Marshal(in)
	if string(b) != string(bIn) {
		t.Fatalf("expected unchanged payload, got %s", string(b))
	}
}

func TestToolOutputTruncatesLargePayload(t *testing.T) {
	t.Cleanup(func() { _ = tooloutput.CleanupProjectTemp(testProjectHex) })
	lim := tooloutput.Limits{MaxBytes: 200, MaxLines: 10}
	svc := testToolOutputService(lim)
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = strings.Repeat("x", 20)
	}
	in := map[string]any{"output": strings.Join(lines, "\n"), "exit": 0}
	out := svc.Apply(in, tooloutput.Meta{SessionID: "sess-trunc", ToolCallID: "call-trunc", ToolName: "shell"})
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	if m["truncated"] != true {
		t.Fatal("expected truncated true")
	}
	spillPath, _ := m["spill_path"].(string)
	if spillPath == "" {
		t.Fatal("expected spill_path")
	}
	if _, err := os.Stat(spillPath); err != nil {
		t.Fatalf("spill file missing: %v", err)
	}
	msg, _ := m["output"].(string)
	want := tooloutput.FormatTruncatedMessage(spillPath)
	if msg != want {
		t.Fatalf("output message\n got: %q\nwant: %q", msg, want)
	}
	if !svc.SpillGenerated() {
		t.Fatal("expected spill generated flag")
	}
}

func TestToolOutputLimitsFromConfig(t *testing.T) {
	lim := tooloutput.LimitsFromConfig(&config.Root{
		ToolOutput: config.ToolOutput{MaxBytes: 4096, MaxLines: 512},
	})
	if lim.MaxBytes != 4096 || lim.MaxLines != 512 {
		t.Fatalf("unexpected limits: %+v", lim)
	}
	def := tooloutput.LimitsFromConfig(nil)
	if def.MaxBytes != tooloutput.DefaultMaxBytes || def.MaxLines != tooloutput.DefaultMaxLines {
		t.Fatalf("unexpected defaults: %+v", def)
	}
}

func TestToolOutputSpillWritesJSON(t *testing.T) {
	t.Cleanup(func() { _ = tooloutput.CleanupProjectTemp(testProjectHex) })
	svc := testToolOutputService(tooloutput.Limits{MaxBytes: 50, MaxLines: 5})
	out := svc.Apply(map[string]any{"k": strings.Repeat("v", 200)}, tooloutput.Meta{SessionID: "s", ToolCallID: "c", ToolName: "mcp"})
	m := out.(map[string]any)
	spillPath, _ := m["spill_path"].(string)
	if !strings.HasSuffix(spillPath, ".json") {
		t.Fatalf("expected .json spill, got %q", spillPath)
	}
	dir := filepath.Dir(spillPath)
	if dir == "" {
		t.Fatal("empty spill dir")
	}
}

func TestFormatTruncatedMessage(t *testing.T) {
	got := tooloutput.FormatTruncatedMessage("/tmp/spill.json")
	if !strings.Contains(got, "---TRUNCATED---") {
		t.Fatalf("missing marker: %q", got)
	}
	if !strings.Contains(got, "full output at /tmp/spill.json") {
		t.Fatalf("missing path line: %q", got)
	}
}
