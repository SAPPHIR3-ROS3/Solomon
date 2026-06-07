package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

const sampleMultiLegacyToolCalls = `<tool_calls>
<tool name="shell">
<intent>Run unit tests</intent>
<args>{"command":"go test ./internal/..."}</args>
</tool>
<tool name="readFile">
<intent>Inspect config</intent>
<args>{"path":"config.toml"}</args>
</tool>
</tool_calls>`

func allowedBuildLegacyTools() map[string]struct{} {
	names := []string{"shell", "readFile", "editFile", "subagent", "loadSkill", "searchSkill", "fetchWeb", "webSearch"}
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}
	return out
}

func assertMalformedLegacyTool(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, tooling.ErrMalformedLegacyTool) {
		t.Fatalf("expected ErrMalformedLegacyTool, got %v", err)
	}
}

func writeStreamParts(t *testing.T, w *tooling.LegacyStreamWriter, parts ...string) error {
	t.Helper()
	for _, p := range parts {
		_, err := w.Write([]byte(p))
		if err != nil && !errors.Is(err, tooling.ErrLegacyToolBlockComplete) {
			return err
		}
		if errors.Is(err, tooling.ErrLegacyToolBlockComplete) {
			return nil
		}
	}
	return nil
}

func TestParseToolCallsBlock_single(t *testing.T) {
	block := `<tool_calls>
<tool name="shell">
<intent>Run tests</intent>
<args>{"command":"go test ./..."}</args>
</tool>
</tool_calls>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 1 {
		t.Fatalf("got %d invocations", len(invs))
	}
	if invs[0].Name != "shell" {
		t.Fatalf("name=%q", invs[0].Name)
	}
	var m map[string]string
	if err := json.Unmarshal(invs[0].Args, &m); err != nil {
		t.Fatal(err)
	}
	if m["command"] != "go test ./..." || m["intent"] != "Run tests" {
		t.Fatalf("args=%v", m)
	}
}

func TestParseToolCallsBlock_qwenJSONToolCall(t *testing.T) {
	block := `<tool_call>{"name":"readFile","arguments":{"path":"main.go"}}</tool_call>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 1 || invs[0].Name != "readFile" {
		t.Fatalf("got %+v", invs)
	}
	var m map[string]string
	if err := json.Unmarshal(invs[0].Args, &m); err != nil {
		t.Fatal(err)
	}
	if m["path"] != "main.go" {
		t.Fatalf("args=%v", m)
	}
}

func TestParseToolCallsBlock_glaiveFunctionCall(t *testing.T) {
	block := `<functioncall>{"name":"shell","arguments":{"command":"echo hi","intent":"test"}}</functioncall>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 1 || invs[0].Name != "shell" {
		t.Fatalf("got %+v", invs)
	}
}

func TestParseToolCallsBlock_mixedToolCallCloseTags(t *testing.T) {
	block := `<tool_calls>
<tool name="shell">
<args>{"command":"rg onboard"}</args>
</tool>
<tool_call>
<tool name="readFile">
<args>{"path":"onboard.go"}</args>
</tool_call>
</tool_calls>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 2 || invs[0].Name != "shell" || invs[1].Name != "readFile" {
		t.Fatalf("got %+v", invs)
	}
}

func TestExtractToolInvocations_qwenWithoutWrapper(t *testing.T) {
	text := "I'll read the file.\n\n<tool_call>{\"name\":\"readFile\",\"arguments\":{\"path\":\"a.go\"}}</tool_call>"
	invs, err := tooling.ExtractToolInvocations(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 1 || invs[0].Name != "readFile" {
		t.Fatalf("got %+v", invs)
	}
}

func TestParseToolCallsBlock_multiple(t *testing.T) {
	invs, err := tooling.ParseToolCallsBlock(sampleMultiLegacyToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 2 {
		t.Fatalf("got %d invocations", len(invs))
	}
	if invs[0].Name != "shell" || invs[1].Name != "readFile" {
		t.Fatalf("names=%q %q", invs[0].Name, invs[1].Name)
	}
}

func TestParseToolCallsBlock_noIntent(t *testing.T) {
	block := `<tool_calls>
<tool name="readFile">
<args>{"path":"main.go"}</args>
</tool>
</tool_calls>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(invs[0].Args, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["intent"]; ok {
		t.Fatalf("unexpected intent: %v", m)
	}
	if m["path"] != "main.go" {
		t.Fatalf("args=%v", m)
	}
}

func TestParseToolCallsBlock_emptyArgsObject(t *testing.T) {
	block := `<tool_calls>
<tool name="shell">
<args>{}</args>
</tool>
</tool_calls>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	if string(invs[0].Args) != "{}" {
		t.Fatalf("args=%s", invs[0].Args)
	}
}

func TestParseToolCallsBlock_extraUnknownArgsPassThrough(t *testing.T) {
	block := `<tool_calls>
<tool name="shell">
<args>{"command":"echo hi","unknownParam":123,"nested":{"a":1}}</args>
</tool>
</tool_calls>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(invs[0].Args, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["unknownParam"]; !ok {
		t.Fatal("expected unknownParam to pass through")
	}
}

func TestParseToolCallsBlock_multilineJSONInArgs(t *testing.T) {
	block := `<tool_calls>
<tool name="editFile">
<intent>Fix block</intent>
<args>{"path":"a.go","oldString":"line1\nline2","newString":"line3\nline4"}</args>
</tool>
</tool_calls>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(invs[0].Args, &m); err != nil {
		t.Fatal(err)
	}
	if m["oldString"] != "line1\nline2" {
		t.Fatalf("oldString=%q", m["oldString"])
	}
}

func TestParseToolCallsBlock_editFileDelete(t *testing.T) {
	block := `<tool_calls>
<tool name="editFile">
<intent>Remove temp file</intent>
<args>{"path":"tmp/old.txt","delete":true}</args>
</tool>
</tool_calls>`
	invs, err := tooling.ParseToolCallsBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 1 || invs[0].Name != "editFile" {
		t.Fatalf("invs=%+v", invs)
	}
	var m map[string]any
	if err := json.Unmarshal(invs[0].Args, &m); err != nil {
		t.Fatal(err)
	}
	if m["delete"] != true || m["path"] != "tmp/old.txt" {
		t.Fatalf("args=%v", m)
	}
}

func TestLegacyToolInvocationSyntax_editFileDeleteExample(t *testing.T) {
	s := prompt.LegacyToolInvocationSyntaxAppend(false)
	if !strings.Contains(s, `"delete": true`) {
		t.Fatalf("expected editFile delete legacy example: %q", s)
	}
}

func TestParseToolCallsBlock_malformedCases(t *testing.T) {
	cases := []struct {
		name  string
		block string
	}{
		{
			name: "invalid json",
			block: `<tool_calls>
<tool name="shell">
<args>{not json}</args>
</tool>
</tool_calls>`,
		},
		{
			name: "missing args",
			block: `<tool_calls>
<tool name="shell">
<intent>Run</intent>
</tool>
</tool_calls>`,
		},
		{
			name: "empty tool name",
			block: `<tool_calls>
<tool name="">
<args>{"command":"x"}</args>
</tool>
</tool_calls>`,
		},
		{
			name: "unclosed tool",
			block: `<tool_calls>
<tool name="shell">
<args>{"command":"x"}</args>
</tool_calls>`,
		},
		{
			name: "empty tool_calls",
			block: `<tool_calls></tool_calls>`,
		},
		{
			name: "args json array",
			block: `<tool_calls>
<tool name="shell">
<args>["a","b"]</args>
</tool>
</tool_calls>`,
		},
		{
			name: "args json string",
			block: `<tool_calls>
<tool name="shell">
<args>"hello"</args>
</tool>
</tool_calls>`,
		},
		{
			name: "unexpected prose inside wrapper",
			block: `<tool_calls>
oops
<tool name="shell">
<args>{"command":"x"}</args>
</tool>
</tool_calls>`,
		},
		{
			name: "unclosed wrapper in block parse",
			block: `<tool_calls>
<tool name="shell">
<args>{"command":"x"}</args>
</tool>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tooling.ParseToolCallsBlock(tc.block)
			assertMalformedLegacyTool(t, err)
		})
	}
}

func TestWriteLabeledTranscriptRendersToolsNotXML(t *testing.T) {
	var buf bytes.Buffer
	msgs := []chatstore.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: sampleMultiLegacyToolCalls},
	}
	commands.WriteLabeledTranscript(&buf, msgs, "gpt-5", false)
	out := buf.String()
	if strings.Contains(out, "<tool_calls>") {
		t.Fatalf("transcript should not show raw XML: %s", out)
	}
	if !strings.Contains(out, "Tool: shell") || !strings.Contains(out, "Tool: readFile") {
		t.Fatalf("expected Tool: lines: %s", out)
	}
	if strings.Contains(out, `readFile({"path"`) {
		t.Fatalf("transcript should use friendly tool lines, not raw JSON args: %s", out)
	}
}

func TestStripLegacyToolBlocks_removesFromReasoning(t *testing.T) {
	text := "planning.\n" + sampleMultiLegacyToolCalls + "\nmore thought"
	got := tooling.StripLegacyToolBlocks(text)
	if strings.Contains(got, "<tool_calls>") || strings.Contains(got, "readFile") {
		t.Fatalf("tool block must be removed from reasoning: %q", got)
	}
	if !strings.Contains(got, "planning.") || !strings.Contains(got, "more thought") {
		t.Fatalf("prose around block should remain: %q", got)
	}
}

func TestLegacyProseOutsideToolCalls(t *testing.T) {
	text := "Here is the plan.\n" + sampleMultiLegacyToolCalls + "\nDone."
	got := tooling.LegacyProseOutsideToolCalls(text)
	if strings.Contains(got, "<tool_calls>") {
		t.Fatalf("prose should not contain XML: %q", got)
	}
	if !strings.Contains(got, "Here is the plan.") {
		t.Fatalf("missing prefix: %q", got)
	}
}

func TestExtractToolInvocations_withProse(t *testing.T) {
	text := "I'll run tests.\n\n" + sampleMultiLegacyToolCalls
	invs, err := tooling.ExtractToolInvocations(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 2 {
		t.Fatalf("got %d", len(invs))
	}
}

func TestExtractToolInvocations_noBlock(t *testing.T) {
	invs, err := tooling.ExtractToolInvocations("plain text only")
	if err != nil {
		t.Fatal(err)
	}
	if invs != nil {
		t.Fatalf("expected nil, got %v", invs)
	}
}

func TestExtractToolInvocations_unclosed(t *testing.T) {
	_, err := tooling.ExtractToolInvocations("prefix <tool_calls><tool name=\"shell\"><args>{}</args></tool>")
	assertMalformedLegacyTool(t, err)
}

func TestExtractToolInvocations_usesFirstBlockOnly(t *testing.T) {
	text := sampleMultiLegacyToolCalls + "\n\n" + strings.Replace(sampleMultiLegacyToolCalls, "shell", "fetchWeb", 1)
	invs, err := tooling.ExtractToolInvocations(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 2 {
		t.Fatalf("got %d invocations from first block only", len(invs))
	}
	if invs[0].Name != "shell" {
		t.Fatalf("first tool=%q", invs[0].Name)
	}
}

func TestExtractToolInvocations_missingRequiredParamStillParses(t *testing.T) {
	block := `<tool_calls>
<tool name="shell">
<args>{"intent":"Run something"}</args>
</tool>
</tool_calls>`
	invs, err := tooling.ExtractToolInvocations(block)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(invs[0].Args, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["command"]; ok {
		t.Fatal("command should be absent")
	}
}

func TestLegacyStreamWriter_completeBlock(t *testing.T) {
	var out strings.Builder
	format := func(name string, args json.RawMessage) []string {
		return []string{"FMT:" + name}
	}
	w := tooling.NewLegacyStreamWriter(&out, format, allowedBuildLegacyTools())
	prefix := "Hello\n"
	block := `<tool_calls>
<tool name="shell">
<intent>Run</intent>
<args>{"command":"echo hi"}</args>
</tool>
</tool_calls>`
	_, err := w.Write([]byte(prefix + block + "\nignored"))
	if !errors.Is(err, tooling.ErrLegacyToolBlockComplete) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(out.String(), "Hello") {
		t.Fatal("missing prefix")
	}
	if !strings.Contains(out.String(), "FMT:shell") {
		t.Fatalf("out=%q", out.String())
	}
	if !w.DisplayRendered() {
		t.Fatal("expected display rendered")
	}
	if len(w.Invocations()) != 1 {
		t.Fatalf("invs=%v", w.Invocations())
	}
	if w.TruncatedContent() != prefix+block {
		t.Fatalf("truncated=%q", w.TruncatedContent())
	}
}

func TestLegacyStreamWriter_splitOpenTag(t *testing.T) {
	var out strings.Builder
	w := tooling.NewLegacyStreamWriter(&out, nil, allowedBuildLegacyTools())
	if err := writeStreamParts(t, w, "before ", "<tool", "_calls>", `<tool name="shell"><args>{"command":"x"}</args></tool></tool_calls>`); err != nil {
		t.Fatal(err)
	}
	if len(w.Invocations()) != 1 {
		t.Fatalf("invs=%d", len(w.Invocations()))
	}
	if !strings.HasPrefix(out.String(), "before ") {
		t.Fatalf("out=%q", out.String())
	}
}

func TestLegacyStreamWriter_splitCloseTag(t *testing.T) {
	var out strings.Builder
	w := tooling.NewLegacyStreamWriter(&out, nil, allowedBuildLegacyTools())
	block := `<tool_calls><tool name="shell"><args>{"command":"x"}</args></tool></tool_calls>`
	parts := []string{"pre ", "<tool_calls><tool name=\"shell\"><args>{\"command\":\"x\"}</args></tool></tool", "_calls>"}
	if err := writeStreamParts(t, w, parts...); err != nil {
		t.Fatal(err)
	}
	if len(w.Invocations()) != 1 {
		t.Fatalf("invs=%d", len(w.Invocations()))
	}
	if w.TruncatedContent() != "pre "+block {
		t.Fatalf("truncated=%q", w.TruncatedContent())
	}
}

func TestLegacyStreamWriter_splitPerByte(t *testing.T) {
	var out strings.Builder
	w := tooling.NewLegacyStreamWriter(&out, nil, allowedBuildLegacyTools())
	payload := "x" + sampleMultiLegacyToolCalls
	for i := 0; i < len(payload); i++ {
		if _, err := w.Write([]byte{payload[i]}); err != nil && !errors.Is(err, tooling.ErrLegacyToolBlockComplete) {
			t.Fatalf("byte %d: %v", i, err)
		}
	}
	if len(w.Invocations()) != 2 {
		t.Fatalf("invs=%d", len(w.Invocations()))
	}
}

func TestLegacyStreamWriter_splitMidSecondTool(t *testing.T) {
	var out strings.Builder
	w := tooling.NewLegacyStreamWriter(&out, nil, allowedBuildLegacyTools())
	part1 := `<tool_calls>
<tool name="shell">
<args>{"command":"a"}</args>
</tool>
<tool name="read`
	part2 := `File">
<args>{"path":"b.go"}</args>
</tool>
</tool_calls>`
	if err := writeStreamParts(t, w, part1, part2); err != nil {
		t.Fatal(err)
	}
	if len(w.Invocations()) != 2 {
		t.Fatalf("invs=%d", len(w.Invocations()))
	}
}

func TestLegacyStreamWriter_hasOpenToolCalls(t *testing.T) {
	w := tooling.NewLegacyStreamWriter(&strings.Builder{}, nil, allowedBuildLegacyTools())
	if w.HasOpenToolCalls() {
		t.Fatal("expected false initially")
	}
	if _, err := w.Write([]byte("start <tool_calls><tool name=\"shell\">")); err != nil {
		t.Fatal(err)
	}
	if !w.HasOpenToolCalls() {
		t.Fatal("expected open block before close")
	}
}

func TestLegacyStreamWriter_malformedBlockReturnsError(t *testing.T) {
	w := tooling.NewLegacyStreamWriter(&strings.Builder{}, nil, allowedBuildLegacyTools())
	_, err := w.Write([]byte(`<tool_calls><tool name="shell"><args>{bad</args></tool></tool_calls>`))
	assertMalformedLegacyTool(t, err)
}

func TestLegacyStreamWriter_ignoresAfterComplete(t *testing.T) {
	var out strings.Builder
	w := tooling.NewLegacyStreamWriter(&out, nil, allowedBuildLegacyTools())
	block := `<tool_calls><tool name="shell"><args>{"command":"x"}</args></tool></tool_calls>`
	if err := writeStreamParts(t, w, block); err != nil {
		t.Fatal(err)
	}
	n, err := w.Write([]byte("more text should be ignored"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len("more text should be ignored") {
		t.Fatalf("n=%d", n)
	}
	if strings.Contains(out.String(), "more text") {
		t.Fatalf("out=%q", out.String())
	}
}

func TestLegacyStreamWriter_flushEmitsHeldOutsideSuffix(t *testing.T) {
	var out strings.Builder
	w := tooling.NewLegacyStreamWriter(&out, nil, allowedBuildLegacyTools())
	if _, err := w.Write([]byte("hello <tool")); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello <tool" {
		t.Fatalf("out=%q", out.String())
	}
	if w.HasOpenToolCalls() {
		t.Fatal("partial open tag alone must not count as open block")
	}
}

func TestLegacyStreamWriter_multiToolFormatted(t *testing.T) {
	var out strings.Builder
	var names []string
	format := func(name string, args json.RawMessage) []string {
		names = append(names, name)
		return []string{"FMT:" + name}
	}
	w := tooling.NewLegacyStreamWriter(&out, format, allowedBuildLegacyTools())
	if err := writeStreamParts(t, w, sampleMultiLegacyToolCalls); err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "shell" || names[1] != "readFile" {
		t.Fatalf("names=%v", names)
	}
}

func TestValidateLegacyToolLines(t *testing.T) {
	if err := tooling.ValidateLegacyToolLines("no tools here"); err != nil {
		t.Fatal(err)
	}
	if err := tooling.ValidateLegacyToolLines(`<tool_calls><tool name="shell"><args>{bad</args></tool></tool_calls>`); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateInvocationNames_unknownTool(t *testing.T) {
	invs := []tooling.Invocation{{Name: "notARealTool", Args: json.RawMessage(`{}`)}}
	err := tooling.ValidateInvocationNames(invs, allowedBuildLegacyTools())
	if !errors.Is(err, tooling.ErrUnknownLegacyTool) {
		t.Fatalf("got %v", err)
	}
}

func TestLegacyStreamWriter_unknownToolName(t *testing.T) {
	block := `<tool_calls><tool name="notARealTool"><args>{}</args></tool></tool_calls>`
	w := tooling.NewLegacyStreamWriter(&strings.Builder{}, nil, allowedBuildLegacyTools())
	_, err := w.Write([]byte(block))
	if !errors.Is(err, tooling.ErrUnknownLegacyTool) {
		t.Fatalf("got %v", err)
	}
}

func TestUserFacingLegacyToolError_specificMessages(t *testing.T) {
	block := `<tool_calls>
oops
<tool name="shell">
<args>{"command":"x"}</args>
</tool>
</tool_calls>`
	_, err := tooling.ParseToolCallsBlock(block)
	assertMalformedLegacyTool(t, err)
	msg := tooling.UserFacingLegacyToolError(err)
	if strings.Contains(msg, "Use <tool_calls> with") {
		t.Fatalf("generic suffix leaked: %q", msg)
	}
	if !strings.Contains(msg, "stray text") && !strings.Contains(msg, "only <tool>") {
		t.Fatalf("want specific outside-tags message, got %q", msg)
	}

	wrapped := fmt.Errorf("after 1 attempt(s): %w", err)
	msg2 := tooling.UserFacingLegacyToolError(wrapped)
	if strings.Contains(msg2, "after 1 attempt") {
		t.Fatalf("unwrap failed: %q", msg2)
	}
}

func TestLegacyToolsCommand(t *testing.T) {
	cfg := &config.Root{}
	var saved bool
	buf := &bytes.Buffer{}
	d := testDeps(nil)
	d.Cfg = cfg
	d.Out = buf
	d.SaveCfg = func() error { saved = true; return nil }

	if err := commands.LegacyTools(d, []string{"legacytools", "on"}); err != nil || !cfg.Tools.Legacy || cfg.Tools.LegacyForce || !saved {
		t.Fatalf("on: err=%v legacy=%v force=%v saved=%v", err, cfg.Tools.Legacy, cfg.Tools.LegacyForce, saved)
	}
	if !strings.Contains(buf.String(), "legacy tools: ON, force: OFF") {
		t.Fatalf("msg: %q", buf.String())
	}

	saved = false
	buf.Reset()
	if err := commands.LegacyTools(d, []string{"legacytools", "force", "on"}); err != nil || !cfg.Tools.Legacy || !cfg.Tools.LegacyForce || !saved {
		t.Fatalf("force on: err=%v legacy=%v force=%v saved=%v", err, cfg.Tools.Legacy, cfg.Tools.LegacyForce, saved)
	}

	saved = false
	buf.Reset()
	if err := commands.LegacyTools(d, []string{"legacytools", "off"}); err != nil || cfg.Tools.Legacy || cfg.Tools.LegacyForce || !saved {
		t.Fatalf("off: err=%v legacy=%v force=%v saved=%v", err, cfg.Tools.Legacy, cfg.Tools.LegacyForce, saved)
	}
}

func TestCursorToolsCommand(t *testing.T) {
	cfg := &config.Root{}
	var saved bool
	buf := &bytes.Buffer{}
	d := testDeps(nil)
	d.Cfg = cfg
	d.Out = buf
	d.SaveCfg = func() error { saved = true; return nil }

	if err := commands.CursorTools(d, []string{"cursortools", "on"}); err == nil {
		t.Fatal("expected error without Cursor API configured")
	}

	cfg.Providers = map[string]*config.Provider{
		config.ProviderNameCursorAPI: {
			Name:     config.ProviderNameCursorAPI,
			AuthKind: config.AuthKindCursorAPI,
			BaseURL:  "http://127.0.0.1:8766/v1/",
			APIKey:   "cursor-key",
		},
	}
	saved = false
	buf.Reset()
	if err := commands.CursorTools(d, []string{"cursortools", "on"}); err != nil || !cfg.Tools.CursorInternalTools || !saved {
		t.Fatalf("on: err=%v cursor_internal=%v saved=%v", err, cfg.Tools.CursorInternalTools, saved)
	}
	if !strings.Contains(buf.String(), "cursor native tools: on") {
		t.Fatalf("msg: %q", buf.String())
	}

	saved = false
	buf.Reset()
	if err := commands.CursorTools(d, []string{"cursortools"}); err != nil || cfg.Tools.CursorInternalTools || !saved {
		t.Fatalf("toggle off: err=%v cursor_internal=%v saved=%v", err, cfg.Tools.CursorInternalTools, saved)
	}
}