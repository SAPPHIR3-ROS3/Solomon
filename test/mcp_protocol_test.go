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
	"sync"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPManagerConnectsUsingJuly2026Protocol(t *testing.T) {
	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "july-test", Version: "1"}, nil)
	sdkmcp.AddTool(mcpServer, &sdkmcp.Tool{Name: "ping", Description: "ping"}, func(context.Context, *sdkmcp.CallToolRequest, map[string]any) (*sdkmcp.CallToolResult, struct{}, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "pong"}}}, struct{}{}, nil
	})

	var (
		mu              sync.Mutex
		methods         []string
		protocolHeaders []string
	)
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return mcpServer
	}, &sdkmcp.StreamableHTTPOptions{Stateless: true})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		var request struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &request); err == nil && request.Method != "" {
			mu.Lock()
			methods = append(methods, request.Method)
			protocolHeaders = append(protocolHeaders, r.Header.Get("Mcp-Protocol-Version"))
			mu.Unlock()
			if request.Method == "initialize" {
				http.Error(w, "legacy initialize is not supported by this test server", http.StatusBadRequest)
				return
			}
		}
		mcpHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(httpServer.Close)

	mgr := mcp.NewManager(context.Background(), &mcp.Config{Servers: []mcp.ServerConfig{{
		Name: "modern",
		Type: mcp.TransportStreamableHTTP,
		URL:  httpServer.URL,
	}}}, io.Discard)
	t.Cleanup(func() {
		if err := mgr.Close(); err != nil {
			t.Errorf("close MCP manager: %v", err)
		}
	})

	if !mgr.HasTool("MCP.modern.ping") {
		t.Fatalf("July 2026 MCP server tool was not registered: %v", mgr.OpenAITools())
	}
	result, err := mgr.CallTool(context.Background(), "MCP.modern.ping", json.RawMessage(`{"intent":"verify July 2026 MCP"}`))
	if err != nil {
		t.Fatalf("call July 2026 MCP tool: %v", err)
	}
	switch value := result.(type) {
	case string:
		if value != "pong" {
			t.Fatalf("MCP tool result = %#v, want %q", result, "pong")
		}
	case map[string]any:
		content, ok := value["content"].([]any)
		if !ok || len(content) != 1 || content[0] != "pong" {
			t.Fatalf("MCP tool result = %#v, want content [pong]", result)
		}
	default:
		t.Fatalf("MCP tool result = %#v, want text result", result)
	}

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(methods, ",")
	if !strings.Contains(joined, "server/discover") {
		t.Fatalf("MCP request methods = %v, want server/discover", methods)
	}
	if strings.Contains(joined, "initialize") {
		t.Fatalf("MCP request methods = %v, did not expect legacy initialize", methods)
	}
	for _, version := range protocolHeaders {
		if version != "2026-07-28" {
			t.Fatalf("MCP protocol headers = %v, want only 2026-07-28", protocolHeaders)
		}
	}
}

func TestMCPManagerSupportsLegacySSETransport(t *testing.T) {
	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "sse-test", Version: "1"}, nil)
	mcpServer.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method == "server/discover" {
				return nil, fmt.Errorf("legacy SSE server does not implement server/discover")
			}
			return next(ctx, method, req)
		}
	})
	mcpServer.AddTool(&sdkmcp.Tool{Name: "ping", Description: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "pong"}}}, nil
	})
	sseHandler := sdkmcp.NewSSEHandler(func(*http.Request) *sdkmcp.Server {
		return mcpServer
	}, nil)
	httpServer := httptest.NewServer(sseHandler)
	t.Cleanup(httpServer.Close)

	mgr := mcp.NewManager(context.Background(), &mcp.Config{Servers: []mcp.ServerConfig{{
		Name: "legacy",
		Type: mcp.TransportSSE,
		URL:  httpServer.URL,
	}}}, io.Discard)
	t.Cleanup(func() {
		if err := mgr.Close(); err != nil {
			t.Errorf("close legacy SSE MCP manager: %v", err)
		}
	})

	if !mgr.HasTool("MCP.legacy.ping") {
		t.Fatalf("legacy SSE MCP tool was not registered: %v", mgr.OpenAITools())
	}
	result, err := mgr.CallTool(context.Background(), "MCP.legacy.ping", json.RawMessage(`{"intent":"verify legacy SSE MCP"}`))
	if err != nil || result != "pong" {
		t.Fatalf("legacy SSE CallTool() = %#v, %v", result, err)
	}
}
