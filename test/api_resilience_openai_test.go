package test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func openaiResilienceOKSSE() string {
	chunk := `{"id":"chatcmpl-resilience","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`
	stop := `{"id":"chatcmpl-resilience","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`
	usage := `{"id":"chatcmpl-resilience","object":"chat.completion.chunk","created":1,"model":"test","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
	return "data: " + chunk + "\n\ndata: " + stop + "\n\ndata: " + usage + "\n\ndata: [DONE]\n\n"
}

func mockOpenAIResilienceClient(t *testing.T, calls *atomic.Int32) openai.Client {
	t.Helper()
	return openai.NewClient(
		option.WithBaseURL("http://openai-resilience.test/v1"),
		option.WithAPIKey("test-key"),
		option.WithMaxRetries(0),
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			_ = req
			_ = next
			n := calls.Add(1)
			if n == 1 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unavailable","type":"server_error","code":"503"}}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(openaiResilienceOKSSE())),
			}, nil
		}),
	)
}

func TestResilientOpenAI_StreamTurn_503Then200(t *testing.T) {
	var calls atomic.Int32
	client := mockOpenAIResilienceClient(t, &calls)
	inner := &llm.OpenAIBackend{Client: client}
	policy := config.APIResiliencePolicy{
		MaxRetries:  3,
		BaseDelay:   2 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
		CircuitOpen: time.Minute,
	}
	host := "openai-resilience.test"
	rb := llm.NewResilientBackend(inner, host, policy, llm.NewCircuitRegistry())
	turn, err := rb.StreamTurn(context.Background(), llm.TurnRequest{
		Model:    "gpt-test",
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

func TestResilientOpenAI_ClassifiesOpenAIErrorAsRetryable(t *testing.T) {
	var calls atomic.Int32
	client := mockOpenAIResilienceClient(t, &calls)
	inner := &llm.OpenAIBackend{Client: client}
	circuits := llm.NewCircuitRegistry()
	host := "openai-resilience.test"
	policy := config.APIResiliencePolicy{
		MaxRetries:  1,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
		Jitter:      false,
		CircuitOpen: time.Hour,
	}
	rb := llm.NewResilientBackend(inner, host, policy, circuits)
	_, err := rb.StreamTurn(context.Background(), llm.TurnRequest{
		Model:    "gpt-test",
		Messages: []chatstore.Message{{Role: "user", Content: "hi"}},
	}, io.Discard, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error with max_retries=1")
	}
	if !circuits.IsOpen(host) {
		t.Fatal("expected circuit open after single failed attempt")
	}
	circuits.Reset(host)
	calls.Store(1)
	turn, err := rb.StreamTurn(context.Background(), llm.TurnRequest{
		Model:    "gpt-test",
		Messages: []chatstore.Message{{Role: "user", Content: "hi"}},
	}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Content != "ok" {
		t.Fatalf("content: %q", turn.Content)
	}
	if circuits.IsOpen(host) {
		t.Fatal("circuit should stay closed after success reset")
	}
}
