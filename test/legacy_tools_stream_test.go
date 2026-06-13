package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

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
	stopCursorSidecar(t)
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