package test

import (
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestNativeBridgeToolCorrectionUserMsg_orchestrateFirst(t *testing.T) {
	msg := agentruntime.NativeBridgeToolCorrectionUserMsgForTest()
	for _, bad := range []string{"readFile, editFile, and shell via function", "Do not emit <tool_calls>"} {
		if strings.Contains(msg, bad) {
			t.Fatalf("unexpected stale phrase %q in %q", bad, msg)
		}
	}
	for _, want := range []string{"orchestrate", "searchTools", "subagent", "native API tool_calls"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in %q", want, msg)
		}
	}
	if strings.Contains(msg, "<tool_calls> XML or native") {
		t.Fatalf("must not offer XML/native ambiguity in %q", msg)
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

func TestCursorProxyDirectoryCorrections(t *testing.T) {
	for _, name := range []string{"LS", "ls", "ListDir", "list_dir", "listDir", "mcp:LS", "mcp:listDir"} {
		msg := agentruntime.CursorProxyToolCorrectionMessageForTest([]string{name})
		if !strings.Contains(msg, "sdk.ListDir") || strings.Contains(msg, "sdk.Glob") {
			t.Fatalf("%s: wrong directory recovery: %s", name, msg)
		}
	}
}

func TestCursorProxyChatRestrictedCorrections(t *testing.T) {
	for _, names := range [][]string{{"docsRetrieval"}, {"switchMode"}, {"fetchWeb"}, {}} {
		for _, blocked := range []string{"Read", "LS", "ListDir", "WebSearch", "ApplyPatch", "apply_patch", "Await", "GenerateImage", "CallMcpTool", "mcp:editFile", "mcp:external", "AskQuestion", "browser_navigate", "fetchWeb:missing_intent"} {
			msg := agentruntime.ChatProxyToolCorrectionMessageForTest([]string{blocked}, names)
			for _, bad := range []string{"orchestrate", "searchTools", "subagent", "searchSkill", "loadSkill"} {
				if strings.Contains(msg, bad) {
					t.Fatalf("%v / %s suggests absent %s: %s", names, blocked, bad, msg)
				}
			}
			for _, name := range []string{"docsRetrieval", "switchMode", "fetchWeb", "webSearch", "deepResearch", "researchStatus"} {
				present := false
				for _, allowed := range names {
					if name == allowed {
						present = true
					}
				}
				// A blocked label may name an unavailable tool; it must not be suggested as recovery.
				recovery := strings.TrimPrefix(msg, "Blocked by Solomon proxy: "+blocked+".")
				if !present && strings.Contains(recovery, name) {
					t.Fatalf("%v / %s suggests absent %s: %s", names, blocked, name, msg)
				}
			}
		}
	}
	for _, name := range []string{"fetchWeb", "webSearch"} {
		blocked := "WebFetch"
		if name == "webSearch" {
			blocked = "WebSearch"
		}
		msg := agentruntime.ChatProxyToolCorrectionMessageForTest([]string{blocked}, []string{name})
		if !strings.Contains(msg, "Use the native "+name) {
			t.Fatal(msg)
		}
	}
}

func TestCursorProxyRuntimeChatFallbackAndDisplay(t *testing.T) {
	r := &agentruntime.Runtime{Mode: "chat", Prov: &config.Provider{AuthKind: config.AuthKindCursorAPI}}
	content := "hello\n[error] Cursor internal tool call blocked by Solomon proxy: ApplyPatch\ntail"
	cleaned, fallback := r.StripCursorProxyInlineErrorsForTest(content)
	if cleaned != "hello\ntail" {
		t.Fatalf("cleaned: %q", cleaned)
	}
	for _, msg := range []string{fallback, r.ToolInvocationCorrectionUserMsgForTest(), r.ProxyCorrectionScreenMessageForTest()} {
		for _, want := range []string{"CHAT mode", "switchMode", "fetchWeb", "webSearch"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("missing %s: %s", want, msg)
			}
		}
		for _, bad := range []string{"orchestrate", "searchTools", "subagent"} {
			if strings.Contains(msg, bad) {
				t.Fatalf("agent guidance %s in chat: %s", bad, msg)
			}
		}
	}
	r.Mode = "agent"
	if !strings.Contains(r.ToolInvocationCorrectionUserMsgForTest(), "orchestrate") {
		t.Fatal("agent correction changed")
	}
	if !strings.Contains(r.ProxyCorrectionScreenMessageForTest(), "searchTools") {
		t.Fatal("agent screen changed")
	}
}
