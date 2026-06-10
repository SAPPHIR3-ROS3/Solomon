package test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
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

func TestFormatToolDisplayLines_editFileLargePatchHeadTail(t *testing.T) {
	body := strings.Repeat("line\n", 120)
	args, _ := json.Marshal(map[string]string{
		"path":      "big.go",
		"oldString": "",
		"newString": body,
		"intent":    "create module",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 22 {
		t.Fatalf("want header + 10 head + truncated + 10 tail, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "big.go") {
		t.Fatalf("want path in header: %q", lines[0])
	}
	if lines[11] == "" || !strings.Contains(lines[11], "TRUNCATED") {
		t.Fatalf("want truncated marker at index 11, got %q", lines[11])
	}
	if strings.Count(strings.Join(lines, "\n"), "line") != 20 {
		t.Fatalf("want exactly 20 content lines shown, got %q", strings.Join(lines, "\n"))
	}
}

func TestFormatToolDisplayLines_editFileDiffStripsCommonLines(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "plan.tmpl",
		"oldString": "same\nold-only\nshared-tail\n",
		"newString": "same\nnew-only\nshared-tail\n",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 3 {
		t.Fatalf("want header + removed + added, got %d: %#v", len(lines), lines)
	}
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "same") || strings.Contains(joined, "shared-tail") {
		t.Fatalf("common lines should be omitted: %q", joined)
	}
	if !strings.Contains(joined, "old-only") || !strings.Contains(joined, "new-only") {
		t.Fatalf("want only differing lines: %q", joined)
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

func TestFormatToolDisplayLines_editFileSkipsEmptyNewBlock(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "before",
		"newString": "",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 2 {
		t.Fatalf("want header + old only, got %d: %#v", len(lines), lines)
	}
}

func TestFormatToolDisplayLines_switchMode(t *testing.T) {
	args, err := json.Marshal(map[string]string{"mode": "agent"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("switchMode", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	plain := termcolor.Plain(lines[0])
	if !strings.Contains(plain, "Tool: switchMode Agent") {
		t.Fatalf("got %q", plain)
	}
	if strings.Contains(plain, "{") {
		t.Fatalf("should not show JSON args: %q", plain)
	}
}

func TestFormatToolDisplayLines_orchestrate(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"test\")\n}\n"
	args, err := json.Marshal(map[string]string{
		"source": src,
		"intent": "count characters",
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	if len(lines) != 9 {
		t.Fatalf("want header + 7 source + footer, got %d: %#v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "orchestrate") || !strings.Contains(lines[0], "Code") {
		t.Fatalf("header: %q", lines[0])
	}
	if !strings.Contains(termcolor.Plain(lines[1]), "1 package main") {
		t.Fatalf("first code line: %q", lines[1])
	}
	if !strings.Contains(termcolor.Plain(lines[6]), "6") || !strings.Contains(termcolor.Plain(lines[6]), "fmt.Println") {
		t.Fatalf("indented line: %q", lines[6])
	}
	if termcolor.Plain(lines[8]) != "Code" {
		t.Fatalf("footer: %q", lines[8])
	}
}

func TestFormatToolDisplayLines_orchestrateExpandsTabs(t *testing.T) {
	src := "func main() {\n\tfmt.Println(\"x\")\n}\n"
	args, err := json.Marshal(map[string]string{"source": src, "intent": "tab test"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	plain := termcolor.Plain(lines[2])
	if strings.Contains(plain, "\t") {
		t.Fatalf("display should not contain tab chars: %q", plain)
	}
	if !strings.Contains(plain, "    fmt.Println") {
		t.Fatalf("tab should expand to 4 spaces: %q", plain)
	}
}

func TestFormatToolDisplayLines_orchestrateTruncatesLongSource(t *testing.T) {
	body := strings.Repeat("fmt.Println(\"x\")\n", 60)
	src := "package main\n\nfunc main() {\n" + body + "}\n"
	args, err := json.Marshal(map[string]string{"source": src, "intent": "stress test"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	if len(lines) != 53 {
		t.Fatalf("want header + 25 + truncated + 25 + footer = 53, got %d", len(lines))
	}
	if !strings.Contains(lines[26], "TRUNCATED") {
		t.Fatalf("truncated marker: %q", lines[26])
	}
}

func TestFormatToolDisplayLines_searchTools(t *testing.T) {
	args, err := json.Marshal(map[string]any{"query": "edit file"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("searchTools", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	if !strings.Contains(lines[0], "searchTools edit file") {
		t.Fatalf("got %q", lines[0])
	}
	if strings.Contains(lines[0], "{") {
		t.Fatalf("should not show JSON args: %q", lines[0])
	}
}

func TestFormatToolDisplayLines_webSearch(t *testing.T) {
	args, err := json.Marshal(map[string]any{
		"query":          "golang context",
		"engine":         "duckduckgo",
		"maxResults":     5,
		"timeoutSeconds": 45,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("webSearch", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	want := "webSearch • duckduckgo (5 result • 45s) | golang context"
	if !strings.Contains(lines[0], want) {
		t.Fatalf("got %q, want substring %q", lines[0], want)
	}

	minimal, err := json.Marshal(map[string]any{"query": "test"})
	if err != nil {
		t.Fatal(err)
	}
	lines = tooling.FormatToolDisplayLines("webSearch", minimal)
	wantMinimal := "webSearch • | test"
	if !strings.Contains(lines[0], wantMinimal) {
		t.Fatalf("defaults: got %q, want substring %q", lines[0], wantMinimal)
	}

	onlyTimeout, err := json.Marshal(map[string]any{"query": "slow", "timeoutSeconds": 60})
	if err != nil {
		t.Fatal(err)
	}
	lines = tooling.FormatToolDisplayLines("webSearch", onlyTimeout)
	if !strings.Contains(lines[0], "webSearch • (60s) | slow") {
		t.Fatalf("custom timeout only: %q", lines[0])
	}

	onlyMax, err := json.Marshal(map[string]any{"query": "wide", "maxResults": 3})
	if err != nil {
		t.Fatal(err)
	}
	lines = tooling.FormatToolDisplayLines("webSearch", onlyMax)
	if !strings.Contains(lines[0], "webSearch • (3 result) | wide") {
		t.Fatalf("custom maxResults only: %q", lines[0])
	}
}

func TestFormatToolDisplayLines_editFileDelete(t *testing.T) {
	args, err := json.Marshal(map[string]any{
		"path":   "obsolete.go",
		"delete": true,
		"intent": "remove dead code",
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	if !strings.Contains(lines[0], "removed obsolete.go") || strings.Contains(lines[0], "(delete)") {
		t.Fatalf("unexpected delete display: %q", lines[0])
	}
}

func TestFormatToolResultDisplayLines_editFileSuccessSilent(t *testing.T) {
	for _, payload := range []string{
		`{"ok":true,"action":"edited"}`,
		`{"ok":true,"action":"deleted"}`,
		`{"ok":true,"action":"created_or_overwrite"}`,
	} {
		lines := tooling.FormatToolResultDisplayLines("editFile", payload)
		if len(lines) != 0 {
			t.Fatalf("payload %s: want no result line, got %v", payload, lines)
		}
	}
}

func TestFormatToolResultDisplayLines_editFileFailureShowsReason(t *testing.T) {
	payload := `{"ok":false,"reason":"oldString not found"}`
	lines := tooling.FormatToolResultDisplayLines("editFile", payload)
	if len(lines) != 1 || !strings.Contains(lines[0], "oldString not found") {
		t.Fatalf("lines: %v", lines)
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
