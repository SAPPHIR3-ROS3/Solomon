package agentruntime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestPrintCursorNativeToolEvent_runningRead(t *testing.T) {
	var buf bytes.Buffer
	r := &Runtime{
		Out: &buf,
		Cfg: &config.Root{Tools: config.Tools{CursorInternalTools: true}},
	}
	r.printCursorNativeToolEvent(`{"name":"Read","status":"running","args":{"path":"main.go"}}`)
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
	r := &Runtime{Out: &buf}
	r.printCursorNativeToolEvent(`{"name":"Shell","status":"completed","result":{"output":"ok"}}`)
	out := buf.String()
	if !strings.Contains(out, "Shell (cursor)") {
		t.Fatalf("expected cursor label: %q", out)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected result preview: %q", out)
	}
}

func TestCursorNativeToolsEnabled(t *testing.T) {
	r := &Runtime{
		Cfg: &config.Root{Tools: config.Tools{CursorInternalTools: true}},
	}
	if r.cursorNativeToolsEnabled() {
		t.Fatal("expected false without cursor provider")
	}
}
