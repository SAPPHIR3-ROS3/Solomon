package test

import (
	"bytes"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/turnloop"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestSubagentDoneSystemMessageFormat(t *testing.T) {
	msg := agentruntime.SubagentDoneSystemMessageForTest("rispondi-ok", "abc123")
	plain := termcolor.Plain(termcolor.FormatSystemBlock(msg))
	if !strings.Contains(plain, "subagent rispondi-ok done") {
		t.Fatalf("missing title line: %q", plain)
	}
	if !strings.Contains(plain, "\tabc123") {
		t.Fatalf("missing indented hash: %q", plain)
	}
	if strings.Contains(plain, "resume=") {
		t.Fatalf("old resume format: %q", plain)
	}
}

func TestWriteSystemDeferredDuringStreaming(t *testing.T) {
	var buf bytes.Buffer
	turnloop.EnterStreaming()
	turnloop.WriteSystemDeferred(&buf, "subagent x done\n\thash")
	if buf.Len() != 0 {
		t.Fatalf("should queue while streaming, got %q", buf.String())
	}
	turnloop.LeaveStreaming(&buf)
	if !strings.Contains(termcolor.Plain(buf.String()), "subagent x done") {
		t.Fatalf("expected flush after stream end: %q", buf.String())
	}
}
