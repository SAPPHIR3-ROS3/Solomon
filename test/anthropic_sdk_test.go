package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

func TestAnthropicSDKThinkingToolContinuation(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "ambient-key-must-not-be-sent")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "ambient-token-must-not-be-sent")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/proxy/v1/messages" || r.URL.Query().Get("route") != "private" {
			t.Errorf("URL=%s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer subscription-token" || r.Header.Get("x-api-key") != "" {
			t.Error("SDK mixed ambient credentials with provider credentials")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		tools := body["tools"].([]any)
		if tools[0].(map[string]any)["name"] != "Read" {
			t.Errorf("tool=%v", tools[0])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			fmt.Fprint(w, anthropicSSEBody(
				map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 10, "cache_read_input_tokens": 5, "cache_creation_input_tokens": 3}}},
				map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "thinking", "thinking": "", "signature": ""}},
				map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": "  original thinking\n"}},
				map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "signature_delta", "signature": "opaque-signature"}},
				map[string]any{"type": "content_block_stop", "index": 0},
				map[string]any{"type": "content_block_start", "index": 1, "content_block": map[string]any{"type": "tool_use", "id": "toolu_read", "name": "Read", "input": map[string]any{}}},
				map[string]any{"type": "content_block_delta", "index": 1, "delta": map[string]any{"type": "input_json_delta", "partial_json": `{"path":"README.md"}`}},
				map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 7}, "delta": map[string]any{"stop_reason": "tool_use"}},
			))
			return
		}
		messages := body["messages"].([]any)
		assistant := messages[1].(map[string]any)["content"].([]any)
		thinking := assistant[0].(map[string]any)
		if thinking["signature"] != "opaque-signature" || thinking["thinking"] != "  original thinking\n" {
			t.Errorf("signed thinking changed: %#v", thinking)
		}
		if assistant[1].(map[string]any)["name"] != "Read" {
			t.Error("history tool name was not mapped with tools[]")
		}
		fmt.Fprint(w, anthropicSSEBody(map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "done"}}, map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}}))
	}))
	defer server.Close()
	backend := llm.NewAnthropicBackendWithClient(server.URL+"/proxy/v1/messages?route=private", llm.AnthropicAuthFromOAuthBearer("subscription-token"), server.Client())
	req := llm.TurnRequest{Model: "claude-sonnet-4-6", Messages: []chatstore.Message{{Role: "user", Content: "read the readme"}}, Tools: []llm.ToolDef{{Name: "read", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}}}}
	turn, err := backend.StreamTurn(context.Background(), req, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != "read" || turn.ToolCalls[0].Arguments != `{"path":"README.md"}` {
		t.Fatalf("tools=%+v", turn.ToolCalls)
	}
	if turn.Usage.PromptTokens != 15 || turn.Usage.CachedPromptTokens != 5 || turn.Usage.CacheCreationPromptTokens != 3 {
		t.Fatalf("usage=%+v", turn.Usage)
	}
	if len(turn.AnthropicThinking) != 1 {
		t.Fatalf("thinking=%s", turn.AnthropicThinking)
	}
	// Round-trip through the persisted message schema before the tool result.
	stored := chatstore.Message{Role: "assistant", AnthropicThinking: turn.AnthropicThinking, ToolCalls: []chatstore.ToolCall{{ID: turn.ToolCalls[0].ID, Name: turn.ToolCalls[0].Name, Arguments: turn.ToolCalls[0].Arguments}}}
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	var restored chatstore.Message
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	req.Messages = append(req.Messages, restored, chatstore.Message{Role: "tool", ToolCallID: "toolu_read", Content: "readme text"})
	turn, err = backend.StreamTurn(context.Background(), req, io.Discard, llm.StreamOpts{})
	if err != nil || turn.Content != "done" {
		t.Fatalf("turn=%+v err=%v", turn, err)
	}
}

func TestAnthropicSDKStreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantEOF    bool
	}{
		{"truncated", "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n", true},
		{"error event", "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"overloaded\"}}\n\n", false},
		{"malformed event", "event: content_block_delta\ndata: {broken json}\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("key"))
			_, _, err := backend.StreamText(context.Background(), llm.SimpleCompletionRequest{Model: "test", User: "hello"}, io.Discard, llm.StreamOpts{})
			if err == nil {
				t.Fatal("stream failure silently accepted")
			}
			if tc.wantEOF && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestAnthropicSDKRetryAfterAndCredentialIsolation(t *testing.T) {
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "ambient-subscription-token")
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("x-api-key") != "provider-key" || r.Header.Get("Authorization") != "" {
			t.Error("API key request leaked bearer")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)
	}))
	defer srv.Close()
	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("provider-key"))
	_, err := backend.CompleteText(context.Background(), llm.SimpleCompletionRequest{Model: "test", User: "hello"})
	var providerErr *transport.ProviderHTTPError
	if !errors.As(err, &providerErr) || providerErr.StatusCode != 429 || providerErr.RetryAfter != 7*time.Second {
		t.Fatalf("error=%v", err)
	}
	if requests != 1 {
		t.Fatalf("SDK retried independently: requests=%d", requests)
	}
}

func TestAnthropicSDKCanceledStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: ping\ndata: {\"type\":\"ping\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("key"))
	_, _, err := backend.StreamText(ctx, llm.SimpleCompletionRequest{Model: "test", User: "hello"}, io.Discard, llm.StreamOpts{})
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline")) {
		t.Fatalf("error=%v", err)
	}
}

func TestAnthropicSDKLongLivedBackendResolvesEachRequest(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer token-%d", calls) {
			t.Error("request used stale subscription token")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			io.WriteString(w, `{"data":[{"id":"claude-sonnet-4-6","type":"model"}],"has_more":false}`)
			return
		}
		io.WriteString(w, `{"content":[{"type":"text","text":"ok"}]}`)
	}))
	defer srv.Close()
	backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromOAuthBearer("initial-token")).WithTokenProvider(func(ctx context.Context) (string, error) { calls++; return fmt.Sprintf("token-%d", calls), nil })
	for i := 0; i < 2; i++ {
		if _, err := backend.CompleteText(context.Background(), llm.SimpleCompletionRequest{Model: "test", User: "hello"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := backend.ListModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("token resolved %d times", calls)
	}
}
