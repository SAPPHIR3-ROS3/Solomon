package test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func TestOpenAICacheUsageStreaming(t *testing.T) {
	for _, tc := range []struct {
		name, usage string
		read, write int64
	}{
		{"missing", `"prompt_tokens":100`, -1, -1},
		{"zero", `"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":0},"cache_creation_input_tokens":0`, 0, 0},
		{"positive", `"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":72,"cache_write_tokens":8}`, 72, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := openai.NewClient(option.WithAPIKey("test"), option.WithMaxRetries(0), option.WithMiddleware(func(_ *http.Request, _ option.MiddlewareNext) (*http.Response, error) {
				sse := strings.Replace(openaiResilienceOKSSE(), `"prompt_tokens":3,"completion_tokens":2,"total_tokens":5`, tc.usage+`,"completion_tokens":2,"total_tokens":102`, 1)
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(sse))}, nil
			}))
			backend := &llm.OpenAIBackend{Client: client}
			turn, err := backend.StreamTurn(context.Background(), llm.TurnRequest{Model: "test"}, io.Discard, llm.StreamOpts{})
			if err != nil {
				t.Fatal(err)
			}
			if turn.Usage.CachedPromptTokens != tc.read || turn.Usage.CacheCreationPromptTokens != tc.write {
				t.Fatalf("turn usage: %+v", turn.Usage)
			}
			_, usage, err := backend.StreamText(context.Background(), llm.SimpleCompletionRequest{Model: "test", User: "hi"}, io.Discard, llm.StreamOpts{})
			if err != nil {
				t.Fatal(err)
			}
			if usage.CachedPromptTokens != tc.read || usage.CacheCreationPromptTokens != tc.write {
				t.Fatalf("text usage: %+v", usage)
			}
		})
	}
}
