package test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func checkpointContPlain(cp int, branch string) string {
	return checkpoint.FormatCheckpointContinuationPlain(cp, branch)
}

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
	cont := checkpointContPlain(3, "")
	if !strings.Contains(out, cont+"old content\n") {
		t.Fatalf("missing continuation prefix: %q", out)
	}
	if !strings.Contains(out, cont+"new content\n") {
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
	if !strings.Contains(out, checkpointContPlain(1, "")+"./foo\n") {
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
	plain := termcolor.Plain(buf.String())
	if !strings.Contains(plain, "[#002]: Tool: readFile a.go") {
		t.Fatalf("first tool display missing: %s", plain)
	}
	if !strings.Contains(plain, "[#003]: Tool: readFile b.go") {
		t.Fatalf("second tool display missing: %s", plain)
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
	plain := termcolor.Plain(buf.String())
	if !strings.Contains(plain, "[#002]: Tool: editFile x.go") {
		t.Fatalf("header line missing: %s", plain)
	}
	cont := checkpointContPlain(2, "")
	if !strings.Contains(plain, cont+"before") {
		t.Fatalf("oldString continuation missing: %s", plain)
	}
	if !strings.Contains(plain, cont+"after") {
		t.Fatalf("newString continuation missing: %s", plain)
	}
}

func TestFormatToolDisplayLines_editFileSplitsBeforeWrap_writesNoSpuriousContinuations(t *testing.T) {
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
	if strings.Count(out, checkpointContPlain(1, "")+"\n") > 0 {
		t.Fatalf("spurious blank continuation lines: %q", out)
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
	plain := termcolor.Plain(buf.String())
	if strings.Contains(plain, `"content":"package main"`) {
		t.Fatalf("transcript should not dump tool result JSON: %s", plain)
	}
	if !strings.Contains(plain, "Tool: readFile") || !strings.Contains(plain, "x.go") {
		t.Fatalf("want formatted tool lines: %s", plain)
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
	plain := termcolor.Plain(buf.String())
	if !strings.Contains(plain, "[#002]: update test file") {
		t.Fatalf("intent line should have checkpoint with colon: %s", plain)
	}
	if !strings.Contains(plain, "[#002]: Tool: editFile x.go") {
		t.Fatalf("tool header missing: %s", plain)
	}
}
