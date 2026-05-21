package llm

import (
	"encoding/json"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

func TestMessagesForAPI_OnlyLastAssistantKeepsReasoning(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "assistant", Content: "a1", ReasoningText: "think1"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2", ReasoningText: "think2"},
	}
	api := MessagesForAPI(msgs)
	if api[0].ReasoningText != "" {
		t.Fatalf("first assistant should strip reasoning, got %q", api[0].ReasoningText)
	}
	if api[2].ReasoningText != "think2" {
		t.Fatalf("last assistant should keep reasoning, got %q", api[2].ReasoningText)
	}
}

func TestAnthropicMessagesURL(t *testing.T) {
	cases := map[string]string{
		"https://api.anthropic.com":         "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/":        "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/v1":      "https://api.anthropic.com/v1/messages",
		"https://proxy.example/v1/messages": "https://proxy.example/v1/messages",
	}
	for in, want := range cases {
		if got := anthropicMessagesURL(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestBuildAnthropicMessages_ToolResult(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "", ToolCalls: []chatstore.ToolCall{{ID: "t1", Name: "shell", Arguments: `{"cmd":"ls"}`}}},
		{Role: "tool", ToolCallID: "t1", Content: `{"ok":true}`},
	}
	out := buildAnthropicMessages(msgs, nil)
	if len(out) < 3 {
		t.Fatalf("expected >=3 messages, got %d", len(out))
	}
	last := out[len(out)-1]
	if last.Role != "user" {
		t.Fatalf("tool results should be user role, got %s", last.Role)
	}
	raw, err := json.Marshal(last.Content)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid content json: %s", raw)
	}
}

func TestNormalizeAnthropicUsage(t *testing.T) {
	u := normalizeAnthropicUsage(anthropicUsagePayload{
		InputTokens:              100,
		OutputTokens:             50,
		CacheReadInputTokens:     30,
		CacheCreationInputTokens: 10,
	})
	if u.PromptTokens != 130 {
		t.Fatalf("prompt tokens: got %d want 130", u.PromptTokens)
	}
	if u.CachedPromptTokens != 30 {
		t.Fatalf("cached: got %d want 30", u.CachedPromptTokens)
	}
	if u.CacheCreationPromptTokens != 10 {
		t.Fatalf("creation: got %d want 10", u.CacheCreationPromptTokens)
	}
	if u.ResponseTokens != 50 {
		t.Fatalf("response: got %d want 50", u.ResponseTokens)
	}
}
