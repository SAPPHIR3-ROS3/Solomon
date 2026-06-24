package test

import (
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
)

func TestNativeBridgeToolCorrectionUserMsg_orchestrateFirst(t *testing.T) {
	msg := agentruntime.NativeBridgeToolCorrectionUserMsgForTest()
	for _, bad := range []string{"readFile, editFile, and shell via function", "Do not emit <tool_calls>"} {
		if strings.Contains(msg, bad) {
			t.Fatalf("unexpected stale phrase %q in %q", bad, msg)
		}
	}
	for _, want := range []string{"orchestrate", "searchTools", "subagent", "<tool_calls> XML"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in %q", want, msg)
		}
	}
}

func TestCursorProxyToolCorrectionMessage_redirectOrchestrateFirst(t *testing.T) {
	msg := agentruntime.CursorProxyToolCorrectionMessageForTest([]string{"Read", "Shell"})
	for _, want := range []string{"Blocked by Solomon proxy", "Read", "searchTools", "orchestrate", "sdk.ReadFile"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in %q", want, msg)
		}
	}
	for _, bad := range []string{"readFile, editFile", "use Read", "use Shell", "use StrReplace", "use Task"} {
		if strings.Contains(msg, bad) {
			t.Fatalf("unexpected stale phrase %q in %q", bad, msg)
		}
	}
}

func TestCursorProxyToolCorrectionMessage_subagentRedirect(t *testing.T) {
	msg := agentruntime.CursorProxyToolCorrectionMessageForTest([]string{"Task"})
	if !strings.Contains(msg, "native subagent") {
		t.Fatalf("missing native subagent hint in %q", msg)
	}
	if strings.Contains(msg, "use Task") {
		t.Fatalf("must not mention use Task in %q", msg)
	}
}

func TestCursorProxyToolCorrectionMessage_hardDenyOmitsFooter(t *testing.T) {
	msg := agentruntime.CursorProxyToolCorrectionMessageForTest([]string{"AskQuestion"})
	if !strings.Contains(msg, "plain text instead of AskQuestion") {
		t.Fatalf("missing hard-deny hint in %q", msg)
	}
	if strings.Contains(msg, "never Cursor built-ins") {
		t.Fatalf("unexpected old footer in %q", msg)
	}
	if strings.Contains(msg, "searchTools") || strings.Contains(msg, "orchestrate") {
		t.Fatalf("hard-deny should omit orchestrate footer in %q", msg)
	}
}

func TestStripCursorProxyInlineErrors_buildsProxyCorrection(t *testing.T) {
	content := "hello\n[error] Cursor internal tool call blocked by Solomon proxy: StrReplace\ntail"
	cleaned, fallback := agentruntime.StripCursorProxyInlineErrorsForTest(content)
	if cleaned != "hello\ntail" {
		t.Fatalf("cleaned=%q", cleaned)
	}
	if !strings.Contains(fallback, "Blocked by Solomon proxy: StrReplace") {
		t.Fatalf("fallback=%q", fallback)
	}
	if !strings.Contains(fallback, "sdk.ReplaceInFile") {
		t.Fatalf("fallback=%q", fallback)
	}
}
