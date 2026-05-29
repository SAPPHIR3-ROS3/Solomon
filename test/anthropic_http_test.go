package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func anthropicSSEBody(events ...map[string]any) string {
	var b strings.Builder
	for _, ev := range events {
		raw, err := json.Marshal(ev)
		if err != nil {
			panic(err)
		}
		b.WriteString("data: ")
		b.Write(raw)
		b.WriteString("\n\n")
	}
	return b.String()
}

func newAnthropicMockServer(t *testing.T, wantPath string, check func(*http.Request), body string, status int) *httptest.Server {
	t.Helper()
	if status == 0 {
		status = http.StatusOK
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s want POST", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("path: got %s want %s", r.URL.Path, wantPath)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key: got %q want test-key", got)
		}
		if got := r.Header.Get("anthropic-version"); got != llm.AnthropicAPIVersion {
			t.Errorf("anthropic-version: got %q want %s", got, llm.AnthropicAPIVersion)
		}
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		if status == http.StatusOK && body != "" {
			_, _ = io.WriteString(w, body)
		}
	}))
}

func TestAnthropicBackend_StreamText_MockHTTP(t *testing.T) {
	sse := anthropicSSEBody(
		map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"usage": map[string]any{"input_tokens": 42, "cache_read_input_tokens": 5},
			},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "Hello "},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "world"},
		},
		map[string]any{
			"type": "message_delta",
			"usage": map[string]any{
				"input_tokens":                  42,
				"output_tokens":               12,
				"cache_read_input_tokens":     5,
				"cache_creation_input_tokens": 3,
			},
			"delta": map[string]any{"stop_reason": "end_turn"},
		},
	)
	srv := newAnthropicMockServer(t, "/v1/messages", func(r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Accept: got %q want text/event-stream", r.Header.Get("Accept"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["stream"] != true {
			t.Fatalf("stream: got %v want true", body["stream"])
		}
		if body["model"] != "claude-test" {
			t.Fatalf("model: got %v", body["model"])
		}
	}, sse, 0)
	defer srv.Close()

	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("test-key"))
	var visible bytes.Buffer
	text, usage, err := backend.StreamText(context.Background(), llm.SimpleCompletionRequest{
		Model:  "claude-test",
		System: "sys",
		User:   "ping",
	}, &visible, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello world" {
		t.Fatalf("text: got %q want Hello world", text)
	}
	if visible.String() != "Hello world" {
		t.Fatalf("visible: got %q", visible.String())
	}
	if usage.PromptTokens != 47 {
		t.Fatalf("prompt tokens: got %d want 47", usage.PromptTokens)
	}
	if usage.CachedPromptTokens != 5 {
		t.Fatalf("cached: got %d want 5", usage.CachedPromptTokens)
	}
	if usage.CacheCreationPromptTokens != 3 {
		t.Fatalf("cache creation: got %d want 3", usage.CacheCreationPromptTokens)
	}
	if usage.ResponseTokens != 12 {
		t.Fatalf("response tokens: got %d want 12", usage.ResponseTokens)
	}
}

func TestAnthropicBackend_StreamTurn_MockHTTP_TextAndTool(t *testing.T) {
	sse := anthropicSSEBody(
		map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"usage": map[string]any{"input_tokens": 10},
			},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "ok"},
		},
		map[string]any{
			"type":  "content_block_start",
			"index": 1,
			"content_block": map[string]any{
				"type": "tool_use",
				"id":   "toolu_abc",
				"name": "shell",
			},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 1,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": `{"cmd":"ls"}`},
		},
		map[string]any{
			"type":  "message_delta",
			"usage": map[string]any{"output_tokens": 7},
			"delta": map[string]any{"stop_reason": "tool_use"},
		},
	)
	srv := newAnthropicMockServer(t, "/v1/messages", func(r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["stream"] != true {
			t.Fatalf("stream: got %v", body["stream"])
		}
	}, sse, 0)
	defer srv.Close()

	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("test-key"))
	turn, err := backend.StreamTurn(context.Background(), llm.TurnRequest{
		Model: "claude-test",
		Messages: []chatstore.Message{
			{Role: "user", Content: "run ls"},
		},
	}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Content != "ok" {
		t.Fatalf("content: got %q want ok", turn.Content)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("tool calls: got %d want 1", len(turn.ToolCalls))
	}
	tc := turn.ToolCalls[0]
	if tc.ID != "toolu_abc" || tc.Name != "shell" || tc.Arguments != `{"cmd":"ls"}` {
		t.Fatalf("tool call: %+v", tc)
	}
	if turn.Usage.ResponseTokens != 7 {
		t.Fatalf("response tokens: got %d want 7", turn.Usage.ResponseTokens)
	}
}

func TestAnthropicBackend_StreamTurn_LegacyEarlyStopIgnoresToolUse(t *testing.T) {
	block := `<tool_calls><tool name="shell"><args>{"command":"go test"}</args></tool></tool_calls>`
	sse := anthropicSSEBody(
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "pre " + block},
		},
		map[string]any{
			"type":  "content_block_start",
			"index": 1,
			"content_block": map[string]any{
				"type": "tool_use",
				"id":   "toolu_abc",
				"name": "shell",
			},
		},
	)
	srv := newAnthropicMockServer(t, "/v1/messages", nil, sse, 0)
	defer srv.Close()

	var contentOut io.Writer = tooling.NewLegacyStreamWriter(io.Discard, nil, map[string]struct{}{"shell": {}})

	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("test-key"))
	turn, err := backend.StreamTurn(context.Background(), llm.TurnRequest{
		Model:    "claude-test",
		Messages: []chatstore.Message{{Role: "user", Content: "run tests"}},
	}, contentOut, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(turn.ToolCalls) != 0 {
		t.Fatalf("tool calls: got %+v want none after legacy early stop", turn.ToolCalls)
	}
	if !strings.Contains(turn.Content, "<tool_calls>") {
		t.Fatalf("content=%q", turn.Content)
	}
}

func TestAnthropicBackend_CompleteText_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") == "text/event-stream" {
			t.Error("CompleteText should not request SSE")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != false {
			t.Fatalf("stream: got %v want false", body["stream"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Chat title"},
			},
		})
	}))
	defer srv.Close()

	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("test-key"))
	out, err := backend.CompleteText(context.Background(), llm.SimpleCompletionRequest{
		Model:  "claude-test",
		User:   "name this chat",
		System: "title bot",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "Chat title" {
		t.Fatalf("got %q want Chat title", out)
	}
}

func TestAnthropicBackend_StreamTurn_MockHTTP_Error(t *testing.T) {
	srv := newAnthropicMockServer(t, "/v1/messages", nil, `{"error":"invalid"}`, http.StatusUnauthorized)
	defer srv.Close()

	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("test-key"))
	_, err := backend.StreamTurn(context.Background(), llm.TurnRequest{
		Model:    "claude-test",
		Messages: []chatstore.Message{{Role: "user", Content: "x"}},
	}, io.Discard, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error: %v", err)
	}
}

func TestAnthropicBackend_StreamText_ProxyBaseURL_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicSSEBody(
			map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "x"}},
			map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
		))
	}))
	defer srv.Close()
	backend := llm.NewAnthropicBackend(srv.URL+"/v1/messages", llm.AnthropicAuthFromAPIKey("test-key"))
	text, _, err := backend.StreamText(context.Background(), llm.SimpleCompletionRequest{Model: "m", User: "u"}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if text != "x" {
		t.Fatalf("got %q", text)
	}
}

func TestAnthropicBackend_StreamText_OAuthBearer_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer oat-test" {
			t.Fatalf("Authorization: got %q", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != llm.AnthropicOAuthBeta {
			t.Fatalf("anthropic-beta: got %q want %q", got, llm.AnthropicOAuthBeta)
		}
		if got := r.Header.Get("x-api-key"); got != "" {
			t.Fatalf("x-api-key should be empty, got %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicSSEBody(
			map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "ok"}},
			map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
		))
	}))
	defer srv.Close()
	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromOAuthBearer("oat-test"))
	text, _, err := backend.StreamText(context.Background(), llm.SimpleCompletionRequest{Model: "m", User: "u"}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if text != "ok" {
		t.Fatalf("got %q", text)
	}
}
