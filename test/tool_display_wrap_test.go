package test

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

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

func TestWriteToolDisplayLines_intentAndResultVisualWrap(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	first := checkpointContPlain(51, "b")
	intent := termcolor.WrapThinking("intent: " + strings.Repeat("disable automatic scoring and keep only manual assignment from the user ", 3))
	result := termcolor.ToolHeaderLine("orchestrate", "→ "+strings.Repeat("long-result ", 12))
	var buf bytes.Buffer
	tooling.WriteToolDisplayLinesWithWidth(&buf, 51, "b", []string{intent}, 72)
	tooling.WriteToolDisplayLinesWithPrefixesAndWidth(&buf, "[#051b] ", first, []string{result}, 72)

	rows := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(rows) < 4 {
		t.Fatalf("expected wrapped intent and result rows, got %d: %q", len(rows), buf.String())
	}
	plain := make([]string, len(rows))
	for i, row := range rows {
		plain[i] = termcolor.Plain(row)
	}
	resultStarted := false
	for _, row := range plain {
		if strings.HasPrefix(row, "[#051b] Tool: orchestrate") {
			resultStarted = true
			continue
		}
		if resultStarted && strings.HasPrefix(row, "[#051b]") {
			t.Fatalf("wrapped result row repeated checkpoint: %q", row)
		}
	}
	if !strings.Contains(strings.Join(plain, "\n"), first) {
		t.Fatalf("wrapped rows should preserve continuation dots: %q", buf.String())
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
