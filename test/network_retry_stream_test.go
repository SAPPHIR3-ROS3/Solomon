package test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func TestNetworkRetryUnblocksActiveStreamOnRouteChange(t *testing.T) {
	var route atomic.Int32
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `data: {"id":"partial","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`+"\n\n")
			w.(http.Flusher).Flush()
			route.Store(1) // Ethernet becomes preferred while the Wi-Fi stream stalls.
			<-r.Context().Done()
			return
		}
		_, _ = io.WriteString(w, openaiResilienceOKSSE())
	}))
	defer server.Close()
	transport := networkhttp.NewForTest(http.DefaultTransport.(*http.Transport), func() string {
		if route.Load() == 0 {
			return "wifi"
		}
		return "ethernet"
	}, true)
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	b := llm.NewResilientBackend(nil, "route-change-test", config.APIResiliencePolicy{MaxRetries: 2}, llm.NewCircuitRegistry())
	b.BeforeRetry = transport.PrepareRetry
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b.Inner = &llm.OpenAIBackend{Client: openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("offline-test"), option.WithHTTPClient(client), option.WithMaxRetries(0))}
	turn, err := b.StreamTurn(ctx, llm.TurnRequest{Model: "test"}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || turn.Content != "ok" {
		t.Fatalf("calls=%d output=%q", calls.Load(), turn.Content)
	}
}
