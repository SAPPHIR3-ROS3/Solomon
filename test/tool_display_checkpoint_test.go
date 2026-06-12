package test

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

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
	if strings.Count(out, checkpointContPlain(1, "")+"\n") > 0 {
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
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
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
	if termcolor.Plain(lines[0]) != "Tool: editFile obsolete.go" {
		t.Fatalf("unexpected delete display plain: %q", termcolor.Plain(lines[0]))
	}
	if !strings.Contains(lines[0], termcolor.WrapRed("obsolete.go")) {
		t.Fatalf("delete path should be red: %q", lines[0])
	}
}

func TestFormatToolDisplayLines_deleteRemoveRedArg(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	delArgs, _ := json.Marshal(map[string]string{"name": "old.md"})
	lines := tooling.FormatToolDisplayLines("deletePlan", delArgs)
	if !strings.Contains(lines[0], termcolor.WrapRed("old.md")) {
		t.Fatalf("deletePlan arg should be red: %q", lines[0])
	}
	rmArgs, _ := json.Marshal(map[string]string{"sha1": "abc123deadbeef"})
	lines = tooling.FormatToolDisplayLines("removeTodo", rmArgs)
	if !strings.Contains(lines[0], termcolor.WrapRed("abc123deadbeef")) {
		t.Fatalf("removeTodo arg should be red: %q", lines[0])
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

func TestFormatToolResultDisplayLines_addTodoSuccessSilent(t *testing.T) {
	payload := `{"ok":true,"sha":"89f93794061db4a04d4ba0f9c36915d8c073f7ad","status":"not_built"}`
	lines := tooling.FormatToolResultDisplayLines("addTodo", payload)
	if len(lines) != 0 {
		t.Fatalf("want no addTodo result line, got %v", lines)
	}
}

func TestFormatToolResultDisplayLines_addTodoFailureShowsReason(t *testing.T) {
	payload := `{"error":"todo is required"}`
	lines := tooling.FormatToolResultDisplayLines("addTodo", payload)
	if len(lines) != 1 || !strings.Contains(lines[0], "todo is required") {
		t.Fatalf("lines: %v", lines)
	}
}

func TestFormatToolResultDisplayLines_todoListSilent(t *testing.T) {
	payload := `{"abc123":"Do work","def456":"Other"}`
	lines := tooling.FormatToolResultDisplayLines("todoList", payload)
	if len(lines) != 0 {
		t.Fatalf("want no todoList result line, got %v", lines)
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

func TestFormatToolDisplayLines_planTools(t *testing.T) {
	createArgs, _ := json.Marshal(map[string]string{"name": "feat.md", "goal": "Add planning"})
	lines := tooling.FormatToolDisplayLines("createPlan", createArgs)
	if len(lines) != 1 || !strings.Contains(termcolor.Plain(lines[0]), "Tool: createPlan feat.md") {
		t.Fatalf("createPlan: %#v", lines)
	}

	editArgs, _ := json.Marshal(map[string]string{
		"name": "feat.md", "old": "old-only\n", "new": "new-only\n", "intent": "update design",
	})
	lines = tooling.FormatToolDisplayLines("editPlan", editArgs)
	if len(lines) != 3 {
		t.Fatalf("editPlan want header+2 diff lines, got %d: %#v", len(lines), lines)
	}
	if !strings.Contains(termcolor.Plain(lines[0]), "Tool: editPlan feat.md") {
		t.Fatalf("editPlan header: %q", lines[0])
	}

	buildArgs, _ := json.Marshal(map[string]string{"name": "feat.md"})
	lines = tooling.FormatToolDisplayLines("buildPlan", buildArgs)
	if !strings.Contains(termcolor.Plain(lines[0]), "Tool: buildPlan feat.md") {
		t.Fatalf("buildPlan: %#v", lines)
	}

	todoArgs, _ := json.Marshal(map[string]string{"name": "feat.md", "todo": "Write tests"})
	lines = tooling.FormatToolDisplayLines("addTodo", todoArgs)
	if !strings.Contains(termcolor.Plain(lines[0]), "Tool: addTodo feat.md") {
		t.Fatalf("addTodo: %#v", lines)
	}
}

func TestFormatToolDisplayLines_orchestrateNestedParensColored(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	src := "func f(a (int)) { x[(y)] }\n"
	args, _ := json.Marshal(map[string]string{"source": src, "intent": "paren colors"})
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	if len(lines) < 2 {
		t.Fatal("expected orchestrate lines")
	}
	code := lines[1]
	if !strings.Contains(code, "\x1b[") {
		t.Fatalf("expected ANSI highlights: %q", code)
	}
	if strings.Count(code, "\x1b[") < 4 {
		t.Fatalf("expected multiple color spans for nested parens: %q", code)
	}
}

func TestHighlightGoLine_stringLiteralInteriorColored(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	got := tooling.HighlightGoLineForTest(`import "fmt"`)
	if !termcolor.Enabled() {
		t.Skip("colors disabled")
	}
	if termcolor.Plain(got) != `import "fmt"` {
		t.Fatalf("plain: %q", got)
	}
	wantLit := termcolor.GoString(`"fmt"`)
	if !strings.Contains(got, wantLit) {
		t.Fatalf("full string literal should be one pink span; got %q want substring %q", got, wantLit)
	}
}

func TestWriteToolDisplayLines_orchestrateCodeVisualWrap(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	longLine := `out, _ := sdk.Shell('grep -rn "Task\|cursor\|Cursor" internal/agent/tools/ --include="*.go" | grep -i "bridge\|normalize\|task\|subagent" | head -30', "find cursor bridge")`
	src := "package main\n\nfunc main() {\n\t" + longLine + "\n}\n"
	args, _ := json.Marshal(map[string]string{"source": src, "intent": "wrap test"})
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	var buf bytes.Buffer
	tooling.WriteToolDisplayLinesWithWidth(&buf, 1, "", lines, 72)
	out := buf.String()
	rows := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(rows) < 4 {
		t.Fatalf("expected wrapped orchestrate code rows, got %d: %q", len(rows), out)
	}
	numRE := regexp.MustCompile(`^\d+ `)
	var codeRows []string
	var contRaw string
	cont := checkpointContPlain(1, "")
	for _, row := range rows[1:] {
		plain := termcolor.Plain(row)
		if !strings.HasPrefix(plain, cont) {
			continue
		}
		body := strings.TrimPrefix(plain, cont)
		if strings.Contains(body, "sdk.Shell") && len(codeRows) == 0 {
			codeRows = append(codeRows, body)
		} else if len(codeRows) == 1 && !numRE.MatchString(strings.TrimLeft(body, " ")) {
			codeRows = append(codeRows, body)
			contRaw = row
			break
		}
	}
	if len(codeRows) < 2 {
		t.Fatalf("expected at least 2 visual rows for long line, got %v in %q", codeRows, out)
	}
	if contRaw == "" || !strings.Contains(termcolor.Plain(contRaw), cont) {
		t.Fatalf("wrap continuation should keep checkpoint dots: %q", contRaw)
	}
	first := codeRows[0]
	second := codeRows[1]
	if !numRE.MatchString(first) {
		t.Fatalf("first wrapped row should keep line number: %q", first)
	}
	if numRE.MatchString(strings.TrimLeft(second, " ")) {
		t.Fatalf("continuation row should not repeat line number: %q", second)
	}
	gutter := numRE.FindStringIndex(first)
	if gutter == nil {
		t.Fatalf("unexpected first row shape: %q", first)
	}
	if !strings.HasPrefix(second, strings.Repeat(" ", gutter[1])) {
		t.Fatalf("continuation should align after line-number gutter width %d: first=%q second=%q", gutter[1], first, second)
	}
}

func TestOrchestrateCodeWrapPreservesHighlight(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	line := "    out3, _ := sdk.Shell(`grep -n foo | head -20`, \"find\")"
	styled := tooling.HighlightGoLineForTest(line)
	cut := strings.Index(line, "ead")
	if cut < 0 {
		t.Fatal("test line shape")
	}
	preserved := tooling.SplitStyledRangeForTest(styled, cut, len(line))
	rerun := tooling.HighlightGoLineForTest(line[cut:])
	if preserved == rerun {
		t.Fatalf("wrap split should preserve mid-string highlight; preserved=%q rerun=%q", preserved, rerun)
	}
	if termcolor.Plain(preserved) != line[cut:] {
		t.Fatalf("plain text mismatch: got %q want %q", termcolor.Plain(preserved), line[cut:])
	}
	if eIdx := strings.Index(preserved, "ead"); eIdx <= 0 || !strings.Contains(preserved[:eIdx], "\x1b[") {
		t.Fatalf("continuation slice should carry active ANSI before ead: %q", preserved)
	}
}

func TestWriteToolDisplayLines_orchestrateCodeWrapMidBacktickString(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	longLine := `out3, _ := sdk.Shell(` + "`grep -n \"WriteSession\\|saveSession\\|r\\\\.Session\" internal/agent/runtime/turns.go | head -20`, \"find session saves in turns\")"
	src := "package main\n\nfunc main() {\n\t" + longLine + "\n}\n"
	args, _ := json.Marshal(map[string]string{"source": src, "intent": "mid-backtick wrap"})
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	fullLine := ""
	for _, ln := range lines {
		if strings.Contains(termcolor.Plain(ln), "sdk.Shell") {
			fullLine = ln
			break
		}
	}
	if fullLine == "" {
		t.Fatal("missing shell line in display")
	}
	gutterLen := strings.Index(termcolor.Plain(fullLine), "out3")
	if gutterLen < 0 {
		t.Fatal("unexpected line shape")
	}
	codePlain := termcolor.Plain(fullLine)[gutterLen:]
	codeStyled := tooling.HighlightGoLineForTest(codePlain)
	cut := strings.Index(codePlain, "ead")
	if cut < 0 {
		t.Fatal("expected head split point in test line")
	}
	slice := tooling.SplitStyledRangeForTest(codeStyled, cut, len(codePlain))
	if !strings.Contains(slice, "\x1b[") {
		t.Fatalf("mid-backtick slice should include string color codes: %q", slice)
	}
	var buf bytes.Buffer
	tooling.WriteToolDisplayLinesWithWidth(&buf, 1, "", lines, 72)
	for _, row := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		plain := termcolor.Plain(row)
		if !strings.Contains(plain, "ead -20") {
			continue
		}
		if strings.Count(row, "\x1b[") < 2 {
			t.Fatalf("wrapped row with ead -20 should keep string highlighting: %q", row)
		}
		return
	}
	t.Fatalf("no wrapped row with ead -20 in %q", buf.String())
}

func TestWriteToolDisplayLines_toolHeaderVisualWrap(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	longBody := "subagent-persistence.md • Creare chatstore/subsession.go con ReadSubSession, WriteSubSession, ListSubSessions, DeleteSubSession"
	header := termcolor.ToolHeaderLine("addTodo", longBody)
	var buf bytes.Buffer
	tooling.WriteToolDisplayLinesWithWidth(&buf, 45, "b", []string{header}, 72)
	out := buf.String()
	rows := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(rows) < 2 {
		t.Fatalf("expected wrapped tool header rows, got %d: %q", len(rows), out)
	}
	if !strings.HasPrefix(termcolor.Plain(rows[0]), "[#045b]: Tool: addTodo") {
		t.Fatalf("first row should keep checkpoint prefix: %q", rows[0])
	}
	cont := checkpointContPlain(45, "b")
	if !strings.HasPrefix(termcolor.Plain(rows[1]), cont) {
		t.Fatalf("wrapped tool header should use continuation dots: %q", rows[1])
	}
	if strings.Contains(termcolor.Plain(rows[1]), "[#045b]") {
		t.Fatalf("wrapped row should not repeat checkpoint: %q", rows[1])
	}
}

func TestWriteToolDisplayLines_toolHeaderWrapBulletAligned(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	body := "subagent-persistence.md • Estendere runtime/nested.go (NestedRunConfig/NestedRunResult), ID generation helpers"
	header := termcolor.ToolHeaderLine("addTodo", body)
	wantPlain := termcolor.Plain(header)
	var buf bytes.Buffer
	tooling.WriteToolDisplayLinesWithWidth(&buf, 47, "b", []string{header}, 72)
	rows := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(rows) < 2 {
		t.Fatalf("expected wrapped rows, got %d", len(rows))
	}
	gotPlain := reconstructWrappedToolPlain(rows)
	if gotPlain != wantPlain {
		t.Fatalf("wrapped plain mismatch:\nwant %q\ngot  %q", wantPlain, gotPlain)
	}
	cont := checkpointContPlain(47, "b")
	start1 := toolRowBodyStartCells(termcolor.Plain(rows[1]))
	if start1 != utf8.RuneCountInString(cont) {
		t.Fatalf("wrapped row should start body right after dots, got cell %d: %q", start1, rows[1])
	}
	if strings.HasSuffix(strings.TrimRight(termcolor.Plain(rows[0]), " "), "ID g") {
		t.Fatalf("should not orphan mid-word fragment before wrap: %q", rows[0])
	}
}

func reconstructWrappedToolPlain(rows []string) string {
	var b strings.Builder
	for i, row := range rows {
		plain := termcolor.Plain(row)
		if i == 0 {
			if idx := strings.Index(plain, ": "); idx >= 0 {
				plain = plain[idx+2:]
			}
		} else {
			plain = plain[toolRowBodyStartByte(plain):]
		}
		b.WriteString(plain)
	}
	return b.String()
}

func toolRowBodyStartByte(plain string) int {
	if strings.HasPrefix(plain, "[#") {
		if idx := strings.Index(plain, ": "); idx >= 0 {
			return idx + 2
		}
	}
	i := 0
	for i < len(plain) {
		r, sz := utf8.DecodeRuneInString(plain[i:])
		if r == '.' || r == ' ' {
			i += sz
			continue
		}
		break
	}
	return i
}

func toolRowBodyStartCells(plain string) int {
	return utf8.RuneCountInString(plain[:toolRowBodyStartByte(plain)])
}

func TestWriteToolDisplayLines_editLineVisualWrap(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	long := "- Ogni file <= 500 LoC; nessun nuovo modulo Go esterno; backward compat runNestedWithConfig subagent_registry.go"
	styled := termcolor.WrapEditFileNewStringLine(long)
	var buf bytes.Buffer
	tooling.WriteToolDisplayLinesWithWidth(&buf, 43, "b", []string{
		termcolor.ToolHeaderLine("editPlan", "subagent-persistence.md"),
		styled,
	}, 60)
	out := buf.String()
	rows := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(rows) < 3 {
		t.Fatalf("expected visual wrap into multiple rows, got %d: %q", len(rows), out)
	}
	for i, row := range rows[1:] {
		if !strings.Contains(row, "\x1b[K") {
			t.Fatalf("wrapped edit row %d missing full-line background: %q", i+1, row)
		}
	}
	if strings.Contains(rows[1], "subagen\n") || strings.Contains(out, "subagen\n") {
		t.Fatalf("should not break mid-word: %q", out)
	}
	cont := checkpointContPlain(43, "b")
	if !strings.HasPrefix(termcolor.Plain(rows[1]), cont) {
		t.Fatalf("first edit row should use continuation prefix: %q", rows[1])
	}
	if !strings.HasPrefix(termcolor.Plain(rows[2]), cont) {
		t.Fatalf("wrapped edit continuation should use dots prefix: %q", rows[2])
	}
}

func TestWrapEditFileLineExtendsBackground(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	got := termcolor.WrapEditFileOldStringLine("removed")
	if !strings.Contains(got, "\x1b[K") {
		t.Fatalf("edit line should clear to EOL for full-row background: %q", got)
	}
}
