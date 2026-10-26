package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func TestCodexMiddlewareUsesConfiguredNetworkClient(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"version":"0.999.0"}`)
	}))
	defer registry.Close()
	previous := codex.CodexVersionRegistryURL
	codex.CodexVersionRegistryURL = registry.URL
	defer func() { codex.CodexVersionRegistryURL = previous }()
	// A bypass must fail locally rather than contact the real upstream.
	originalDefault := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: codexNetworkRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == registry.URL {
			return http.DefaultTransport.RoundTrip(req)
		}
		return nil, fmt.Errorf("request bypassed configured network client: %s", req.URL)
	})}
	defer func() { http.DefaultClient = originalDefault }()

	calls := 0
	client := openai.NewClient(
		option.WithAPIKey("offline-test"), option.WithMaxRetries(0),
		option.WithHTTPClient(&http.Client{Transport: codexNetworkRoundTrip(func(req *http.Request) (*http.Response, error) {
			calls++
			if req.URL.String() != codex.ChatGPTResponsesURL {
				t.Errorf("unexpected upstream URL: %s", req.URL)
			}
			if req.Header.Get("Authorization") != "Bearer offline-test" {
				t.Error("authorization lost")
			}
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"offline upstream"}}`)), Request: req}, nil
		})}),
		codex.WithChatGPTSubMiddleware("account-test"),
	)
	_, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: "test-model", Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hello")},
	})
	if err == nil || calls != 1 {
		t.Fatalf("configured transport calls=%d err=%v", calls, err)
	}
}

type codexNetworkRoundTrip func(*http.Request) (*http.Response, error)

func (f codexNetworkRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
