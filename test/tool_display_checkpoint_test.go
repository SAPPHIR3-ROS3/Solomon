package test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestWriteToolDisplayLines_multilineContinuation(t *testing.T) {
	var buf bytes.Buffer
	lines := []string{
		"Tool: editFile path/to/file.go",
		"old content",
		"new content",
	}
	tooling.WriteToolDisplayLines(&buf, 3, "", lines)
	out := buf.String()
	if !strings.HasPrefix(out, "[#003]: Tool: editFile path/to/file.go\n") {
		t.Fatalf("first line should have checkpoint prefix: %q", out)
	}
	if !strings.Contains(out, "..... old content\n") {
		t.Fatalf("missing continuation prefix: %q", out)
	}
	if !strings.Contains(out, "..... new content\n") {
		t.Fatalf("missing second continuation: %q", out)
	}
}

func TestWriteToolDisplayLines_embeddedNewline(t *testing.T) {
	var buf bytes.Buffer
	tooling.WriteToolDisplayLines(&buf, 1, "", []string{"Tool: shell go test\n./foo"})
	out := buf.String()
	if !strings.HasPrefix(out, "[#001]: Tool: shell go test\n") {
		t.Fatalf("first part should have checkpoint prefix: %q", out)
	}
	if !strings.Contains(out, "..... ./foo\n") {
		t.Fatalf("embedded newline continuation: %q", out)
	}
}

func TestWriteLabeledTranscript_toolCallsUseStoredCheckpoints(t *testing.T) {
	var buf bytes.Buffer
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 0, CpSeqSet: true, Content: "run tools"},
		{
			Role:          "assistant",
			CheckpointSeq: 1,
			CpSeqSet:      true,
			ToolCalls: []chatstore.ToolCall{
				{Name: "readFile", Arguments: `{"path":"a.go"}`, CheckpointSeq: 2, CpSeqSet: true},
				{Name: "readFile", Arguments: `{"path":"b.go"}`, CheckpointSeq: 3, CpSeqSet: true},
			},
		},
	}
	commands.WriteLabeledTranscript(&buf, msgs, "gpt-5", false)
	out := buf.String()
	if !strings.Contains(out, "[#002]: Tool: readFile a.go") {
		t.Fatalf("first tool display missing: %s", out)
	}
	if !strings.Contains(out, "[#003]: Tool: readFile b.go") {
		t.Fatalf("second tool display missing: %s", out)
	}
}

func TestFormatToolDisplayLines_editFileLargePatchSummary(t *testing.T) {
	body := strings.Repeat("line\n", 120)
	args, _ := json.Marshal(map[string]string{
		"path":      "big.go",
		"oldString": "",
		"newString": body,
		"intent":    "create module",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 1 {
		t.Fatalf("want summary line, got %d: %v", len(lines), lines)
	}
	if strings.Contains(lines[0], `"newString"`) || strings.Count(lines[0], "line") > 2 {
		t.Fatalf("want compact summary, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "big.go") || !strings.Contains(lines[0], "write") {
		t.Fatalf("want path and write hint: %q", lines[0])
	}
}

func TestWriteLabeledTranscript_editFileMultilineContinuation(t *testing.T) {
	var buf bytes.Buffer
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "before",
		"newString": "after",
	})
	msgs := []chatstore.Message{
		{Role: "assistant", CheckpointSeq: 1, CpSeqSet: true, ToolCalls: []chatstore.ToolCall{
			{Name: "editFile", Arguments: string(args), CheckpointSeq: 2, CpSeqSet: true},
		}},
	}
	commands.WriteLabeledTranscript(&buf, msgs, "gpt-5", false)
	out := buf.String()
	if !strings.Contains(out, "[#002]: Tool: editFile x.go") {
		t.Fatalf("header line missing: %s", out)
	}
	if !strings.Contains(out, "..... before") {
		t.Fatalf("oldString continuation missing: %s", out)
	}
	if !strings.Contains(out, "..... after") {
		t.Fatalf("newString continuation missing: %s", out)
	}
}

func TestFormatToolDisplayLines_editFileSplitsBeforeWrap(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "line1\nline2\n",
		"newString": "line3\n",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 4 {
		t.Fatalf("want header + 2 old + 1 new lines, got %d: %#v", len(lines), lines)
	}
	var buf bytes.Buffer
	tooling.WriteToolDisplayLines(&buf, 1, "", lines)
	out := buf.String()
	if strings.Count(out, "..... \n") > 0 {
		t.Fatalf("spurious blank continuation lines: %q", out)
	}
}

func TestFormatToolDisplayLines_editFileSkipsEmptyOldBlock(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "",
		"newString": "package main\n",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 2 {
		t.Fatalf("want header + new only, got %d: %#v", len(lines), lines)
	}
}

func TestFormatToolResultDisplayLines_readFileOmitsContent(t *testing.T) {
	payload := `{"path":"TODO.md","total_lines":141,"content":"# TODO\n\nlong body"}`
	lines := tooling.FormatToolResultDisplayLines("readFile", payload)
	if len(lines) != 1 {
		t.Fatalf("lines: %v", lines)
	}
	if strings.Contains(lines[0], "# TODO") || strings.Contains(lines[0], "long body") {
		t.Fatalf("must not echo file body: %q", lines[0])
	}
	if !strings.Contains(lines[0], "TODO.md") || !strings.Contains(lines[0], "141") {
		t.Fatalf("want path and line count: %q", lines[0])
	}
}

func TestWriteLabeledTranscript_toolResultNotRawJSON(t *testing.T) {
	var buf bytes.Buffer
	msgs := []chatstore.Message{
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{
			{ID: "call_1", Name: "readFile", Arguments: `{"path":"x.go"}`},
		}},
		{Role: "tool", ToolCallID: "call_1", Content: `{"path":"x.go","total_lines":3,"content":"package main"}`},
	}
	commands.WriteLabeledTranscript(&buf, msgs, "gpt-5", false)
	out := buf.String()
	if strings.Contains(out, `"content":"package main"`) {
		t.Fatalf("transcript should not dump tool result JSON: %s", out)
	}
	if !strings.Contains(out, "Tool: readFile") || !strings.Contains(out, "x.go") {
		t.Fatalf("want formatted tool lines: %s", out)
	}
}

func TestWriteLabeledTranscript_intentLineHasCheckpoint(t *testing.T) {
	var buf bytes.Buffer
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "before",
		"newString": "after",
		"intent":    "update test file",
	})
	msgs := []chatstore.Message{
		{Role: "assistant", CheckpointSeq: 1, CpSeqSet: true, ToolCalls: []chatstore.ToolCall{
			{Name: "editFile", Arguments: string(args), CheckpointSeq: 2, CpSeqSet: true},
		}},
	}
	commands.WriteLabeledTranscript(&buf, msgs, "gpt-5", false)
	out := buf.String()
	if !strings.Contains(out, "[#002]: update test file") {
		t.Fatalf("intent line should have checkpoint with colon: %s", out)
	}
	if !strings.Contains(out, "[#002]: Tool: editFile x.go") {
		t.Fatalf("tool header missing: %s", out)
	}
}
