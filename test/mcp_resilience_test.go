package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type droppingMCPHandler struct {
	delegate      http.Handler
	dropMethod    string
	dropped       atomic.Bool
	executed      atomic.Int32
	listenerCalls atomic.Int32
}

func (h *droppingMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	var message struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &message)
	if message.Method == "subscriptions/listen" {
		h.listenerCalls.Add(1)
	}
	if message.Method == h.dropMethod {
		if h.dropped.CompareAndSwap(false, true) {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					_ = conn.Close()
					return
				}
			}
			http.Error(w, "connection dropped", http.StatusServiceUnavailable)
			return
		}
		if message.Method == "tools/call" {
			h.executed.Add(1)
		}
	}
	h.delegate.ServeHTTP(w, r)
}

func newResilienceMCPServer(t *testing.T) (*droppingMCPHandler, *httptest.Server) {
	t.Helper()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "resilience-test", Version: "1"}, &sdkmcp.ServerOptions{
		SubscribeHandler: func(context.Context, *sdkmcp.SubscribeRequest) error {
			return nil
		},
		UnsubscribeHandler: func(context.Context, *sdkmcp.UnsubscribeRequest) error {
			return nil
		},
	})
	server.AddTool(&sdkmcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "pong"}}}, nil
	})
	server.AddResource(&sdkmcp.Resource{URI: "memo:///one", Name: "one", MIMEType: "text/plain"}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, MIMEType: "text/plain", Text: "resource"}}}, nil
	})
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{Stateless: true})
	dropping := &droppingMCPHandler{delegate: mcpHandler, dropMethod: "tools/call"}
	httpServer := httptest.NewServer(dropping)
	t.Cleanup(func() {
		// The SDK's stateless subscriptions/listen stream is tied to a long-lived
		// HTTP POST. Closing the client session does not always cancel the server
		// handler before httptest.Server.Close waits for active connections. Follow
		// the SDK's teardown sequence: drop the client sockets, then trigger one
		// final notification so the server's write path observes the disconnect.
		httpServer.CloseClientConnections()
		_ = server.ResourceUpdated(context.Background(), &sdkmcp.ResourceUpdatedNotificationParams{URI: "memo:///one"})
		httpServer.Close()
	})
	return dropping, httpServer
}

func TestMCPManagerRecoversWithoutReplayingUnknownToolCall(t *testing.T) {
	dropping, httpServer := newResilienceMCPServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mgr := mcp.NewManagerWithOptions(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name: "resilience",
		Type: mcp.TransportStreamableHTTP,
		URL:  httpServer.URL,
	}}}, nil, &mcp.ManagerOptions{DisableCatalogSubscriptions: true})
	t.Cleanup(func() { _ = mgr.Close() })

	_, err := mgr.CallTool(ctx, "MCP.resilience.ping", []byte(`{"intent":"test lost MCP response"}`))
	if err == nil || !errors.Is(err, mcp.ErrToolCallOutcomeUnknown) {
		t.Fatalf("first CallTool() error = %v, want unknown outcome", err)
	}
	if got := dropping.executed.Load(); got != 0 {
		t.Fatalf("first dropped tool call reached the server %d times", got)
	}

	result, err := mgr.CallTool(ctx, "MCP.resilience.ping", []byte(`{"intent":"test recovered MCP session"}`))
	if err != nil || result != "pong" {
		t.Fatalf("recovered CallTool() = %#v, %v", result, err)
	}
	if got := dropping.executed.Load(); got != 1 {
		t.Fatalf("recovered tool calls reached the server %d times, want 1", got)
	}
}

func TestMCPManagerAllowsExplicitSafeToolReplay(t *testing.T) {
	dropping, httpServer := newResilienceMCPServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mgr := mcp.NewManagerWithOptions(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name: "retry",
		Type: mcp.TransportStreamableHTTP,
		URL:  httpServer.URL,
	}}}, nil, &mcp.ManagerOptions{
		DisableCatalogSubscriptions: true,
		RetryToolCall: func(string, string, *sdkmcp.Tool, error) bool {
			return true
		},
	})
	t.Cleanup(func() { _ = mgr.Close() })

	result, err := mgr.CallTool(ctx, "MCP.retry.ping", []byte(`{"intent":"test explicitly idempotent MCP call"}`))
	if err != nil || result != "pong" {
		t.Fatalf("explicitly retried CallTool() = %#v, %v", result, err)
	}
	if got := dropping.executed.Load(); got != 1 {
		t.Fatalf("retried tool calls reached the server %d times, want 1", got)
	}
}

func TestMCPManagerReusesOAuthHandlerDuringSessionRecovery(t *testing.T) {
	dropping, httpServer := newResilienceMCPServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var factoryCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mcp-test-token" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mcp"`)
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		dropping.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	httpServer.Close()

	mgr := mcp.NewManagerWithOptions(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name:  "oauth-recovery",
		Type:  mcp.TransportStreamableHTTP,
		URL:   server.URL,
		OAuth: &mcp.OAuthConfig{ClientID: "configured-outside-the-build"},
	}}}, nil, &mcp.ManagerOptions{
		DisableCatalogSubscriptions: true,
		OAuthHandler: func(context.Context, mcp.ServerConfig) (auth.OAuthHandler, error) {
			factoryCalls.Add(1)
			return &staticMCPAuthHandler{}, nil
		},
	})
	t.Cleanup(func() { _ = mgr.Close() })

	_, err := mgr.CallTool(ctx, "MCP.oauth-recovery.ping", []byte(`{"intent":"test OAuth recovery"}`))
	if err == nil || !errors.Is(err, mcp.ErrToolCallOutcomeUnknown) {
		t.Fatalf("first OAuth CallTool() error = %v, want unknown outcome", err)
	}
	result, err := mgr.CallTool(ctx, "MCP.oauth-recovery.ping", []byte(`{"intent":"test cached OAuth handler"}`))
	if err != nil || result != "pong" {
		t.Fatalf("recovered OAuth CallTool() = %#v, %v", result, err)
	}
	if got := factoryCalls.Load(); got != 1 {
		t.Fatalf("OAuth handler factory calls = %d, want 1", got)
	}
}

func TestMCPManagerCanDisableModernCatalogListener(t *testing.T) {
	dropping, httpServer := newResilienceMCPServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mgr := mcp.NewManagerWithOptions(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name: "no-listener",
		Type: mcp.TransportStreamableHTTP,
		URL:  httpServer.URL,
	}}}, nil, &mcp.ManagerOptions{DisableCatalogSubscriptions: true})
	t.Cleanup(func() { _ = mgr.Close() })
	if got := dropping.listenerCalls.Load(); got != 0 {
		t.Fatalf("modern catalog listener calls = %d, want 0", got)
	}
}

func TestMCPManagerRestoresModernSubscriptionAfterSessionRecovery(t *testing.T) {
	dropping, httpServer := newResilienceMCPServer(t)
	dropping.dropMethod = "resources/read"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mgr := mcp.NewManagerWithOptions(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name: "subscription-recovery",
		Type: mcp.TransportStreamableHTTP,
		URL:  httpServer.URL,
	}}}, nil, &mcp.ManagerOptions{DisableCatalogSubscriptions: true})
	t.Cleanup(func() { _ = mgr.Close() })

	if err := mgr.Subscribe(ctx, "subscription-recovery", "memo:///one"); err != nil {
		t.Fatalf("Subscribe() = %v", err)
	}
	initialListeners := dropping.listenerCalls.Load()
	if initialListeners != 1 {
		t.Fatalf("initial resource subscription listeners = %d, want 1", initialListeners)
	}

	result, err := mgr.ReadResource(ctx, "subscription-recovery", "memo:///one")
	if err != nil || result == nil || len(result.Contents) != 1 || result.Contents[0].Text != "resource" {
		t.Fatalf("recovered ReadResource() = %#v, %v", result, err)
	}
	waitForMCPFeatureCondition(t, func() bool {
		return dropping.listenerCalls.Load() >= initialListeners+1
	}, "resource subscription recovery")
}
