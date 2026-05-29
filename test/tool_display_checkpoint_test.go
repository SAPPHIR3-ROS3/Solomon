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
	if !strings.HasPrefix(out, "Tool: editFile path/to/file.go\n") {
		t.Fatalf("first line should have no checkpoint prefix: %q", out)
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
	if !strings.HasPrefix(out, "Tool: shell go test\n") {
		t.Fatalf("first part should have no checkpoint prefix: %q", out)
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
	if !strings.Contains(out, "Tool: readFile a.go") {
		t.Fatalf("first tool display missing: %s", out)
	}
	if !strings.Contains(out, "Tool: readFile b.go") {
		t.Fatalf("second tool display missing: %s", out)
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
	if !strings.Contains(out, "Tool: editFile x.go") {
		t.Fatalf("header line missing: %s", out)
	}
	if !strings.Contains(out, "..... before") {
		t.Fatalf("oldString continuation missing: %s", out)
	}
	if !strings.Contains(out, "..... after") {
		t.Fatalf("newString continuation missing: %s", out)
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
	if !strings.Contains(out, "Tool: editFile x.go") {
		t.Fatalf("tool header missing: %s", out)
	}
}
