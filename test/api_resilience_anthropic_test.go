package test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
)

func TestResilientAnthropic_StreamTurn_503Then200(t *testing.T) {
	var calls atomic.Int32
	sse := anthropicSSEBody(
		map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
		map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "ok"}},
		map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
	)
	srv := httptestNewResilienceAnthropicServer(t, &calls, sse)
	defer srv.Close()

	inner := llm.NewAnthropicBackendWithClient(srv.URL, llm.AnthropicAuthFromAPIKey("test-key"), llm.NewResilientHTTPClient(config.EffectiveAPIResilience(nil)))
	policy := config.APIResiliencePolicy{
		MaxRetries:  3,
		BaseDelay:   2 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
		CircuitOpen: time.Minute,
	}
	rb := llm.NewResilientBackend(inner, llm.HostKeyFromBaseURL(srv.URL), policy, llm.NewCircuitRegistry())
	turn, err := rb.StreamTurn(context.Background(), llm.TurnRequest{
		Model:    "claude-test",
		Messages: []chatstore.Message{{Role: "user", Content: "hi"}},
	}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Content != "ok" {
		t.Fatalf("content: %q", turn.Content)
	}
	if calls.Load() != 2 {
		t.Fatalf("HTTP calls: got %d want 2", calls.Load())
	}
}

func httptestNewResilienceAnthropicServer(t *testing.T, calls *atomic.Int32, okBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":"unavailable"}`)
			return
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, okBody)
	}))
}
