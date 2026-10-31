package test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/acp"
)

type brokenACPWriter struct{}

func (brokenACPWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestACPReportsTransportFailure(t *testing.T) {
	s := acp.New(brokenACPWriter{}, savedFactory)
	err := s.Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`+"\n"))
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("lost transport error: %v", err)
	}
}
func TestACPInvalidEnvelopes(t *testing.T) {
	c := startACP(t, "", savedFactory)
	for _, raw := range []string{`{`, `[]`, `null`, `{"jsonrpc":"2.0","id":null,"method":"initialize"}`, `{"jsonrpc":"2.0","id":1.5,"method":"initialize"}`} {
		if _, err := io.WriteString(c.input, raw+"\n"); err != nil {
			t.Fatal(err)
		}
		if c.next(t)["error"] == nil {
			t.Fatalf("accepted invalid envelope: %s", raw)
		}
	}
	c.init(t)
	id := c.newSession(t, t.TempDir())
	for _, p := range []any{nil, []any{}, map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text"}}}} {
		m, _ := c.wait(t, c.request(t, "session/prompt", p))
		if m["error"] == nil {
			t.Fatalf("accepted invalid params: %v", p)
		}
	}
}
func TestACPValidatesMCP(t *testing.T) {
	for _, raw := range []string{`{"name":"stdio","command":"tool","args":[],"env":[]}`, `{"type":"http","name":"http","url":"https://example.com/mcp","headers":[]}`, `{"type":"sse","name":"events","url":"http://localhost/sse","headers":[]}`} {
		if _, err := acp.ParseMCP([]json.RawMessage{json.RawMessage(raw)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{}`, `{"name":"a","type":"http","url":"file:///tmp/server"}`, `{"name":"a","command":"tool","env":[{"name":"BAD=NAME","value":"x"}]}`} {
		if _, err := acp.ParseMCP([]json.RawMessage{json.RawMessage(raw)}); err == nil {
			t.Fatalf("accepted invalid MCP server: %s", raw)
		}
	}
}
func TestACPCloseCancelsActivePrompt(t *testing.T) {
	c := startACP(t, "", savedFactory)
	c.init(t)
	id := c.newSession(t, t.TempDir())
	prompt := c.request(t, "session/prompt", promptParams(id, "wait"))
	if c.next(t)["method"] != "session/update" {
		t.Fatal("missing prompt notification")
	}
	closeID := c.request(t, "session/close", map[string]any{"sessionId": id})
	response, _ := c.wait(t, prompt)
	if response["result"].(map[string]any)["stopReason"] != "cancelled" {
		t.Fatal("close did not cancel prompt")
	}
	response, _ = c.wait(t, closeID)
	if response["error"] != nil {
		t.Fatal(response)
	}
	c.call(t, "session/resume", map[string]any{"sessionId": id, "cwd": c.call(t, "session/list", map[string]any{})["sessions"].([]any)[0].(map[string]any)["cwd"], "mcpServers": []any{}})
}

func TestACPRequestCancellation(t *testing.T) {
	c := startACP(t, "", savedFactory)
	c.init(t)
	id := c.newSession(t, t.TempDir())
	prompt := c.request(t, "session/prompt", promptParams(id, "wait"))
	if c.next(t)["method"] != "session/update" {
		t.Fatal("missing prompt notification")
	}
	c.send(t, map[string]any{"jsonrpc": "2.0", "method": "$/cancel_request", "params": map[string]any{"requestId": prompt}})
	response, _ := c.wait(t, prompt)
	if response["result"].(map[string]any)["stopReason"] != "cancelled" {
		t.Fatal("request cancellation did not stop prompt")
	}
}
