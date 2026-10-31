package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/acp"
)

type acpClient struct {
	input    *io.PipeWriter
	output   *io.PipeReader
	messages chan map[string]any
	done     chan error
	id       int
}

func startACP(t *testing.T, storage string, factory acp.Factory) *acpClient {
	t.Helper()
	in, w := io.Pipe()
	r, out := io.Pipe()
	s := acp.New(out, factory)
	if storage != "" {
		if err := s.SetStorage(storage); err != nil {
			t.Fatal(err)
		}
	}
	c := &acpClient{input: w, output: r, messages: make(chan map[string]any, 100), done: make(chan error, 1)}
	go func() { c.done <- s.Serve(context.Background(), in); out.Close() }()
	go func() {
		defer close(c.messages)
		dec := json.NewDecoder(r)
		for {
			var m map[string]any
			if dec.Decode(&m) != nil {
				return
			}
			c.messages <- m
		}
	}()
	t.Cleanup(func() { c.stop(t) })
	return c
}
func (c *acpClient) stop(t *testing.T) {
	t.Helper()
	if c.input == nil {
		return
	}
	c.input.Close()
	select {
	case err := <-c.done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Error("ACP server did not stop")
	}
	c.output.Close()
	c.input = nil
}
func (c *acpClient) send(t *testing.T, v any) {
	t.Helper()
	if err := json.NewEncoder(c.input).Encode(v); err != nil {
		t.Fatal(err)
	}
}
func (c *acpClient) request(t *testing.T, method string, p any) int {
	t.Helper()
	c.id++
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": c.id, "method": method, "params": p})
	return c.id
}
func (c *acpClient) next(t *testing.T) map[string]any {
	t.Helper()
	select {
	case m, ok := <-c.messages:
		if !ok {
			t.Fatal("ACP transport closed")
		}
		return m
	case <-time.After(5 * time.Second):
		t.Fatal("ACP response timed out")
	}
	return nil
}
func (c *acpClient) wait(t *testing.T, id int) (map[string]any, []map[string]any) {
	t.Helper()
	var updates []map[string]any
	for {
		m := c.next(t)
		if m["id"] == float64(id) {
			return m, updates
		}
		updates = append(updates, m)
	}
}
func (c *acpClient) call(t *testing.T, method string, p any) map[string]any {
	t.Helper()
	m, _ := c.wait(t, c.request(t, method, p))
	if m["error"] != nil {
		t.Fatalf("%s: %v", method, m["error"])
	}
	return m["result"].(map[string]any)
}
func (c *acpClient) init(t *testing.T) {
	t.Helper()
	c.call(t, "initialize", map[string]any{"protocolVersion": 1})
}
func (c *acpClient) newSession(t *testing.T, cwd string) string {
	t.Helper()
	return c.call(t, "session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}})["sessionId"].(string)
}
func promptParams(id, text string) map[string]any {
	return map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": text}}}
}

type savedRunner struct {
	turns       int
	permission  func(context.Context, acp.Update) error
	lastContent []acp.Content
}

func (r *savedRunner) Close() error                       { return nil }
func (r *savedRunner) Snapshot() (json.RawMessage, error) { return json.Marshal(r.turns) }
func (r *savedRunner) Restore(data json.RawMessage) error { return json.Unmarshal(data, &r.turns) }
func (r *savedRunner) SetPermissionHandler(fn func(context.Context, acp.Update) error) {
	r.permission = fn
}
func (r *savedRunner) Configuration() map[string]any                   { return map[string]any{"configOptions": []any{}} }
func (r *savedRunner) Configure(context.Context, string, string) error { return nil }
func (r *savedRunner) PromptContent(ctx context.Context, blocks []acp.Content, emit func(acp.Update)) error {
	r.lastContent = blocks
	return r.Prompt(ctx, blocks[0].Text, emit)
}
func (r *savedRunner) Prompt(ctx context.Context, text string, emit func(acp.Update)) error {
	if text == "wait" {
		emit(acp.Update{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "waiting"}})
		<-ctx.Done()
		return ctx.Err()
	}
	if text == "tool" {
		if err := r.permission(ctx, acp.Update{"toolCallId": "call-1", "title": "Test tool"}); err != nil {
			return err
		}
	}
	r.turns++
	emit(acp.Update{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": fmt.Sprint(r.turns)}})
	return nil
}
func savedFactory(context.Context, string, []json.RawMessage) (acp.Runner, error) {
	return &savedRunner{}, nil
}
func TestACPPersistentLifecycle(t *testing.T) {
	storage, cwd := t.TempDir(), t.TempDir()
	c := startACP(t, storage, savedFactory)
	c.init(t)
	id := c.newSession(t, cwd)
	if c.call(t, "session/prompt", promptParams(id, "hello"))["stopReason"] != "end_turn" {
		t.Fatal("prompt failed")
	}
	c.stop(t)
	c = startACP(t, storage, savedFactory)
	c.init(t)
	rows := c.call(t, "session/list", map[string]any{"cwd": cwd})["sessions"].([]any)
	if len(rows) != 1 {
		t.Fatal("saved session missing")
	}
	result, updates := c.wait(t, c.request(t, "session/load", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": []any{}}))
	if result["error"] != nil || len(updates) != 2 {
		t.Fatalf("history not replayed: %v %v", result, updates)
	}
	_, updates = c.wait(t, c.request(t, "session/prompt", promptParams(id, "hello again")))
	if !strings.Contains(fmt.Sprint(updates), "text:2") {
		t.Fatalf("runner context not restored: %v", updates)
	}
	c.call(t, "session/close", map[string]any{"sessionId": id})
	_, updates = c.wait(t, c.request(t, "session/resume", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": []any{}}))
	if len(updates) != 0 {
		t.Fatal("resume replayed history")
	}
	c.call(t, "session/delete", map[string]any{"sessionId": id})
	c.call(t, "session/delete", map[string]any{"sessionId": id})
	if len(c.call(t, "session/list", map[string]any{})["sessions"].([]any)) != 0 {
		t.Fatal("deleted session is listed")
	}
}
func TestACPModes(t *testing.T) {
	c := startACP(t, "", savedFactory)
	c.init(t)
	result := c.call(t, "session/new", map[string]any{"cwd": t.TempDir(), "mcpServers": []any{}})
	id := result["sessionId"].(string)
	modes := result["modes"].(map[string]any)
	if modes["currentModeId"] != "code" || len(modes["availableModes"].([]any)) != 2 {
		t.Fatalf("unexpected modes: %v", modes)
	}
	if strings.Contains(fmt.Sprint(result), "Ask") {
		t.Fatalf("removed mode advertised: %v", result)
	}
	for _, method := range []string{"session/set_mode", "session/set_config_option"} {
		response, _ := c.wait(t, c.request(t, method, map[string]any{"sessionId": id, "modeId": "ask", "configId": "mode", "value": "ask"}))
		if response["error"] == nil {
			t.Fatalf("%s accepted removed mode", method)
		}
	}
	for _, mode := range []string{"chat", "code"} {
		c.call(t, "session/set_mode", map[string]any{"sessionId": id, "modeId": mode})
	}
	c.call(t, "session/prompt", promptParams(id, "tool"))
}
func TestACPContentAndValidation(t *testing.T) {
	c := startACP(t, "", savedFactory)
	c.init(t)
	id := c.newSession(t, t.TempDir())
	for _, kind := range []string{"audio", "unknown"} {
		m, _ := c.wait(t, c.request(t, "session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": kind}}}))
		if m["error"] == nil {
			t.Fatalf("accepted unsupported %s", kind)
		}
	}
	c.call(t, "session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "resource", "resource": map[string]any{"uri": "file:///example", "text": "embedded text"}}}})
	m, _ := c.wait(t, c.request(t, "session/new", map[string]any{"cwd": "relative"}))
	if m["error"] == nil {
		t.Fatal("relative cwd accepted")
	}
	m, _ = c.wait(t, c.request(t, "session/prompt", promptParams("missing", "hello")))
	if m["error"].(map[string]any)["code"] != float64(-32002) {
		t.Fatal("wrong unknown session error")
	}
}

func TestACPPreventsConcurrentSessionWriters(t *testing.T) {
	storage, cwd := t.TempDir(), t.TempDir()
	first := startACP(t, storage, savedFactory)
	first.init(t)
	id := first.newSession(t, cwd)
	second := startACP(t, storage, savedFactory)
	second.init(t)
	response, _ := second.wait(t, second.request(t, "session/load", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": []any{}}))
	if response["error"] == nil {
		t.Fatal("two connections acquired the same saved session")
	}
	first.call(t, "session/close", map[string]any{"sessionId": id})
	second.call(t, "session/resume", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": []any{}})
	second.call(t, "session/delete", map[string]any{"sessionId": id})
	if len(first.call(t, "session/list", map[string]any{})["sessions"].([]any)) != 0 {
		t.Fatal("session list retained deleted external session")
	}
}
