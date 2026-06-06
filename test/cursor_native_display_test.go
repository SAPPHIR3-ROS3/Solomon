package test

import (
	"bytes"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestPrintCursorNativeToolEvent_runningRead(t *testing.T) {
	var buf bytes.Buffer
	r := &agentruntime.Runtime{
		Out: &buf,
		Cfg: &config.Root{Tools: config.Tools{CursorInternalTools: true}},
	}
	r.PrintCursorNativeToolEvent(`{"name":"Read","status":"running","args":{"path":"main.go"}}`)
	out := buf.String()
	if !strings.Contains(out, "Read (cursor)") {
		t.Fatalf("expected cursor label: %q", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Fatalf("expected path preview: %q", out)
	}
}

func TestPrintCursorNativeToolEvent_completedShell(t *testing.T) {
	var buf bytes.Buffer
	r := &agentruntime.Runtime{Out: &buf}
	r.PrintCursorNativeToolEvent(`{"name":"Shell","status":"completed","result":{"output":"ok"}}`)
	out := buf.String()
	if !strings.Contains(out, "Shell (cursor)") {
		t.Fatalf("expected cursor label: %q", out)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected result preview: %q", out)
	}
}

func TestCursorNativeToolsEnabled(t *testing.T) {
	r := &agentruntime.Runtime{
		Cfg: &config.Root{Tools: config.Tools{CursorInternalTools: true}},
	}
	if r.CursorNativeToolsEnabled() {
		t.Fatal("expected false without cursor provider")
	}
}
