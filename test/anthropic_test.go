package test

import (
	"net/http"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestMessagesForAPI_OnlyLastAssistantKeepsReasoning(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "assistant", Content: "a1", ReasoningText: "think1"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2", ReasoningText: "think2"},
	}
	api := llm.MessagesForAPI(msgs)
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
		if got := llm.AnthropicMessagesURL(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizeAnthropicUsage(t *testing.T) {
	u := llm.NormalizeAnthropicUsage(llm.AnthropicUsagePayload{
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

func TestAnthropicAuth_applyTo(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	if err != nil {
		t.Fatal(err)
	}
	llm.AnthropicAuthFromAPIKey("sk-test").ApplyTo(req)
	if got := req.Header.Get("x-api-key"); got != "sk-test" {
		t.Fatalf("x-api-key: got %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization should be empty for api key auth, got %q", got)
	}
	req.Header = http.Header{}
	llm.AnthropicAuthFromOAuthBearer("oat-test").ApplyTo(req)
	if got := req.Header.Get("Authorization"); got != "Bearer oat-test" {
		t.Fatalf("Authorization: got %q", got)
	}
	if got := req.Header.Get("anthropic-beta"); got != llm.AnthropicOAuthBeta {
		t.Fatalf("anthropic-beta: got %q want %q", got, llm.AnthropicOAuthBeta)
	}
	if got := req.Header.Get("x-api-key"); got != "" {
		t.Fatalf("x-api-key should be empty for oauth bearer, got %q", got)
	}
}
