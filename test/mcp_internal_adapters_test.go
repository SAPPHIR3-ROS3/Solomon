package test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/weblimit"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLazyInternalAdapterDoesNotDiscoverCatalogs(t *testing.T) {
	logging.LogInit()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "web-test", Version: "1"}, nil)
	server.AddResource(&sdkmcp.Resource{Name: "unused", URI: "test:///unused"}, func(context.Context, *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return &sdkmcp.ReadResourceResult{}, nil
	})
	var calls atomic.Int32
	server.AddTool(&sdkmcp.Tool{Name: "web_search", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		calls.Add(1)
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}}}, nil
	})
	var mu sync.Mutex
	methods := map[string]int{}
	server.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			mu.Lock()
			methods[method]++
			mu.Unlock()
			return next(ctx, method, req)
		}
	})
	var requests atomic.Int32
	delegate := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, &sdkmcp.StreamableHTTPOptions{Stateless: true})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		delegate.ServeHTTP(w, r)
	}))
	defer httpServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < 3; i++ {
		m := solomonmcp.NewManagerForTest(&solomonmcp.Config{Servers: []solomonmcp.ServerConfig{{Name: "parallel-web", Type: solomonmcp.TransportStreamableHTTP, URL: httpServer.URL, Internal: true, Adapter: "parallel", Allow: []string{"web_search"}}}}, &solomonmcp.ManagerOptions{LazyInternalAdapters: true})
		if _, _, err := m.Connect(ctx); err != nil {
			t.Fatal(err)
		}
		if requests.Load() != 0 {
			t.Fatal("chat startup contacted the web provider")
		}
		if i == 2 {
			for j := 0; j < 2; j++ {
				if _, err := m.CallInternalTool(ctx, "parallel-web", "web_search", "search", nil); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := m.CallInternalTool(ctx, "parallel-web", "unexpected", "search", nil); err == nil {
				t.Fatal("unknown adapter tool was accepted")
			}
			before := requests.Load()
			if _, err := m.CallInternalTool(ctx, "parallel-web", "web_fetch", "fetch", nil); err == nil {
				t.Fatal("tool outside the configured allow list was accepted")
			}
			if requests.Load() != before {
				t.Fatal("denied tool contacted the provider")
			}
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("tool calls = %d, want 2", calls.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	for _, method := range []string{"tools/list", "resources/list", "resources/templates/list", "prompts/list", "subscriptions/listen"} {
		if methods[method] != 0 {
			t.Errorf("unnecessary %s calls: %d", method, methods[method])
		}
	}
}

func TestInternalUnknownRateLimitIsScopedToChat(t *testing.T) {
	logging.LogInit()
	for _, rpcError := range []bool{false, true} {
		t.Run(fmt.Sprint(rpcError), func(t *testing.T) {
			server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "limited", Version: "1"}, nil)
			var calls atomic.Int32
			handler := func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				calls.Add(1)
				message := "You've hit the free-tier rate limit for Parallel Search MCP"
				if rpcError {
					return nil, fmt.Errorf("%s", message)
				}
				return &sdkmcp.CallToolResult{IsError: true, Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: message}}}, nil
			}
			for _, name := range []string{"web_search", "web_fetch"} {
				server.AddTool(&sdkmcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, handler)
			}
			var requests atomic.Int32
			delegate := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, &sdkmcp.StreamableHTTPOptions{Stateless: true})
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				delegate.ServeHTTP(w, r)
			}))
			defer httpServer.Close()
			path := filepath.Join(t.TempDir(), "cooldowns.json")
			newRuntime := func(chat string, headers map[string]string) *solomonmcp.Manager {
				m := solomonmcp.NewManagerForTest(&solomonmcp.Config{Servers: []solomonmcp.ServerConfig{{Name: "parallel-web", Type: solomonmcp.TransportStreamableHTTP, URL: httpServer.URL, Headers: headers, Internal: true, Adapter: "parallel"}}}, &solomonmcp.ManagerOptions{LazyInternalAdapters: true, InternalRateLimitPath: path})
				m.SetRateLimitChat(chat)
				if _, _, err := m.Connect(context.Background()); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = m.Close() })
				return m
			}
			first := newRuntime("first", nil)
			if _, err := first.CallInternalTool(context.Background(), "parallel-web", "web_search", "search", nil); err == nil {
				t.Fatal("rate limit did not fail the call")
			}
			before := requests.Load()
			for _, manager := range []*solomonmcp.Manager{first, newRuntime("first", nil)} {
				if _, err := manager.CallInternalTool(context.Background(), "parallel-web", "web_fetch", "fetch", nil); err == nil || !strings.Contains(err.Error(), "suspended") {
					t.Fatalf("expected shared cooldown, got %v", err)
				}
			}
			if requests.Load() != before || calls.Load() != 1 {
				t.Fatal("cooldown still contacted the provider")
			}
			changedKey := newRuntime("first", map[string]string{"x-api-key": "test-key"})
			_, _ = changedKey.CallInternalTool(context.Background(), "parallel-web", "web_search", "search", nil)
			if calls.Load() != 2 {
				t.Fatal("new credentials inherited the old cooldown")
			}
			second := newRuntime("second", nil)
			_, _ = second.CallInternalTool(context.Background(), "parallel-web", "web_search", "search", nil)
			first.SetRateLimitChat("third")
			_, _ = first.CallInternalTool(context.Background(), "parallel-web", "web_search", "search", nil)
			if calls.Load() != 4 {
				t.Fatal("new chat inherited a rate limit with no reset")
			}
			first.SetRateLimitChat("first")
			if err := first.CheckInternalRateLimit(context.Background(), "parallel-web"); !weblimit.IsSuspended(err) {
				t.Fatalf("resumed chat lost its suspension: %v", err)
			}
		})
	}
}

func TestInternalHTTPRateLimitUsesRetryAfter(t *testing.T) {
	logging.LogInit()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	ctx := context.Background()
	manager := solomonmcp.NewManagerForTest(&solomonmcp.Config{Servers: []solomonmcp.ServerConfig{{
		Name: "parallel-web", Type: solomonmcp.TransportStreamableHTTP, URL: server.URL, Internal: true, Adapter: "parallel",
	}}}, &solomonmcp.ManagerOptions{LazyInternalAdapters: true})
	t.Cleanup(func() { _ = manager.Close() })
	if _, _, err := manager.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := manager.CallInternalTool(ctx, "parallel-web", "web_search", "search", nil); err == nil {
		t.Fatal("expected HTTP 429 to fail the call")
	}
	var suspended *weblimit.Suspended
	if err := manager.CheckInternalRateLimit(ctx, "parallel-web"); !errors.As(err, &suspended) || suspended.Until.Before(started.Add(time.Hour)) || suspended.Until.After(time.Now().Add(time.Hour)) {
		t.Fatalf("expected provider retry time, got %v", err)
	}
	manager.SetRateLimitChat("new-chat")
	if _, err := manager.CallInternalTool(ctx, "parallel-web", "web_fetch", "fetch", nil); !weblimit.IsSuspended(err) || requests.Load() != 1 {
		t.Fatalf("expected suspension without replay, got %v and %d HTTP requests", err, requests.Load())
	}
}
