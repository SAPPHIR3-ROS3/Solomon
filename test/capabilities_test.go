package test

import (
	"context"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"strings"
	"testing"
	"time"
)

func capabilityJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCapabilitiesUpdateByIdentity(t *testing.T) {
	var registry chatstore.CapabilityRegistry
	for _, description := range []string{"old", "updated"} {
		registry.Capture("searchTools", capabilityJSON(t, map[string]any{"tools": []map[string]string{
			{"name": "readFile", "description": description, "sdk_call": "ReadFile(path, intent string) (string, error)"},
			{"name": "MCP.server.lookup", "description": description, "sdk_call": "sdk.mcp.lookup(intent, args)", "parameters": "object schema"},
		}, "sdk": map[string]string{"contract": "SDK reference"}}))
	}
	if len(registry.DiscoveredTools) != 2 {
		t.Fatalf("tool count: %d", len(registry.DiscoveredTools))
	}
	for _, tool := range registry.DiscoveredTools {
		if !strings.Contains(string(tool), "updated") || !strings.Contains(string(tool), "sdk_call") {
			t.Fatalf("incomplete tool: %s", tool)
		}
	}
	if !strings.Contains(string(registry.DiscoveredTools["MCP.server.lookup"]), "parameters") {
		t.Fatal("MCP schema lost")
	}
	registry.Capture("searchTools", capabilityJSON(t, map[string]any{"tools": []any{}}))
	for _, body := range []string{"old body", "new body"} {
		registry.Capture("loadSkill", capabilityJSON(t, map[string]string{"name": "Build", "slash": "build", "body": body}))
	}
	registry.Capture("loadSkill", capabilityJSON(t, map[string]string{"error": "not found"}))
	if len(registry.LoadedSkills) != 1 || !strings.Contains(string(registry.LoadedSkills["build"]), "new body") {
		t.Fatal("skill update failed")
	}
}

func TestCompactionRestoresCapabilitiesAcrossRounds(t *testing.T) {
	toolPayload := capabilityJSON(t, map[string]any{"tools": []map[string]string{{"name": "shell", "description": "run command", "sdk_call": "Shell(command, intent string) (string, error)"}}, "sdk": map[string]string{"contract": "SDK reference"}})
	skillPayload := capabilityJSON(t, map[string]string{"name": "Build", "slash": "build", "body": "private skill payload"})
	sess := &chatstore.Session{Messages: []chatstore.Message{
		{Role: "user", Content: "implement feature"},
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "d", Name: "searchTools"}, {ID: "s", Name: "loadSkill"}}},
		{Role: "tool", ToolCallID: "d", Content: toolPayload},
		{Role: "tool", ToolCallID: "s", Content: skillPayload},
		{Role: "assistant", Content: "ready"},
	}}
	for round := 0; round < 3; round++ {
		chatstore.ApplyCompaction(sess, "summary", time.Unix(int64(round+1), 0))
		if len(sess.Messages) != 2 || sess.Messages[1].ContextKind != "capabilities" || sess.Messages[1].Role != "user" {
			t.Fatalf("missing restore message: %+v", sess.Messages)
		}
		if !strings.Contains(sess.Messages[1].Content, "private skill payload") || !strings.Contains(sess.Messages[1].Content, "Shell(command") {
			t.Fatal("restore incomplete")
		}
		projected := chatstore.SummaryMessages(sess.Messages)
		if len(projected) != 1 || strings.Contains(commands.FormatChatTranscript(projected), "private skill") {
			t.Fatal("restore leaked into summary")
		}
		b, err := json.Marshal(sess)
		if err != nil {
			t.Fatal(err)
		}
		var loaded chatstore.Session
		if err := json.Unmarshal(b, &loaded); err != nil {
			t.Fatal(err)
		}
		sess = &loaded
	}
	if len(sess.UncompactedRaw[0].Messages) != 5 {
		t.Fatal("archive changed")
	}
}

func TestSummaryProjectionExcludesCapabilities(t *testing.T) {
	messages := []chatstore.Message{
		{Role: "system", Content: "private system"},
		{Role: "developer", Content: "private developer"},
		{Role: "assistant", Content: "Looking up APIs", ToolCalls: []chatstore.ToolCall{{ID: "d", Name: "searchTools", Arguments: "private query"}, {ID: "s", Name: "loadSkill"}, {ID: "f", Name: "readFile"}}},
		{Role: "tool", ToolCallID: "d", Content: "private tool schemas"},
		{Role: "tool", ToolCallID: "s", Content: "private skill body"},
		{Role: "tool", ToolCallID: "f", Content: "useful source output"},
		{Role: "user", Content: "Apply the agent skill \"Build\":\n\nprivate slash body"},
		{Role: "user", Content: "/skill build fix bug", APIContent: "You must use the following installed skill for this turn.\n\nSkill: \"Build\"\n\n--- SKILL START ---\nprivate forced body\n--- SKILL END ---\n\nUser request:\nfix bug"},
		{Role: "user", Content: "tool_result(" + capabilityJSON(t, map[string]string{"name": "Build", "body": "private legacy body"}) + ")"},
		{Role: "user", ContextKind: "capabilities", Content: "private restored body"},
		{Role: "user", Content: "keep this request"},
	}
	projected := chatstore.SummaryMessages(messages)
	for _, output := range []string{commands.FormatChatTranscript(projected), commands.FormatRetainedMessages(projected)} {
		if strings.Contains(output, "private") || strings.Contains(output, "searchTools") || strings.Contains(output, "loadSkill") {
			t.Fatalf("capability leak: %s", output)
		}
		if !strings.Contains(output, "keep this request") || !strings.Contains(output, "readFile") || !strings.Contains(output, "/skill build fix bug") {
			t.Fatalf("conversation lost: %s", output)
		}
	}
	if len(messages[2].ToolCalls) != 3 {
		t.Fatal("projection mutated original calls")
	}
	var registry chatstore.CapabilityRegistry
	registry.Backfill(messages)
	if len(registry.LoadedSkills) == 0 {
		t.Fatal("slash skill missing")
	}
}

func TestBackfillPreservesCanonicalCapabilities(t *testing.T) {
	var registry chatstore.CapabilityRegistry
	registry.Capture("loadSkill", capabilityJSON(t, map[string]string{"name": "Build", "slash": "build", "body": "complete original body"}))
	registry.Backfill([]chatstore.Message{
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "s", Name: "loadSkill"}}},
		{Role: "tool", ToolCallID: "s", Content: capabilityJSON(t, map[string]string{"name": "Build", "slash": "build", "body": "truncated"})},
	})
	if !strings.Contains(string(registry.LoadedSkills["build"]), "complete original body") {
		t.Fatal("canonical skill overwritten")
	}
}

func TestCapabilitiesPersistDedicatedSection(t *testing.T) {
	var registry chatstore.CapabilityRegistry
	registry.Capture("searchTools", capabilityJSON(t, map[string]any{"tools": []map[string]string{{"name": "shell", "description": "run commands", "sdk_call": "Shell"}}}))
	session := chatstore.Session{Capabilities: registry}
	payload := capabilityJSON(t, session)
	if !strings.Contains(payload, "discovered_tools") {
		t.Fatal("dedicated section missing")
	}
	var loaded chatstore.Session
	if err := json.Unmarshal([]byte(payload), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Capabilities.DiscoveredTools) != 1 {
		t.Fatal("registry lost on reload")
	}
	sub := chatstore.SubSession{Capabilities: registry}
	var loadedSub chatstore.SubSession
	if err := json.Unmarshal([]byte(capabilityJSON(t, sub)), &loadedSub); err != nil {
		t.Fatal(err)
	}
	if len(loadedSub.Capabilities.DiscoveredTools) != 1 {
		t.Fatal("subchat registry lost")
	}
}

func TestCapabilitiesCapturedAutomaticallyByRuntime(t *testing.T) {
	backend := &turnScriptBackend{protocol: llm.ProtocolOpenAI, turns: []llm.AssistantTurnResult{
		{ToolCalls: []llm.AssistantToolCall{{ID: "first", Name: "searchTools", Arguments: searchToolsArgs()}, {ID: "second", Name: "searchTools", Arguments: searchToolsArgs()}}},
		{Content: "done"},
	}}
	rt := newTurnLoopRuntime(t, backend, nil, nil)
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rt.Session.Capabilities.DiscoveredTools) != 1 {
		t.Fatalf("registry not deduplicated: %+v", rt.Session.Capabilities)
	}
	if !strings.Contains(string(rt.Session.Capabilities.DiscoveredTools["shell"]), "Shell(") {
		t.Fatal("SDK signature missing")
	}
	if len(rt.Session.Messages) != 5 || rt.Session.Messages[2].ToolCallID != "first" || rt.Session.Messages[3].ToolCallID != "second" {
		t.Fatal("tool protocol changed")
	}
}
