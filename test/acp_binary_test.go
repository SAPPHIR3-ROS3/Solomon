package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestACPBinaryProviderAndRestore(t *testing.T) {
	name := "solomon"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", binary, "./cmd/solomon")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	var mu sync.Mutex
	var requests []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			io.WriteString(w, `{"object":"list","data":[{"id":"test-model","object":"model"},{"id":"second-model","object":"model"},{"id":"limited-model","supported_reasoning_levels":["low","high"]}]}`)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			http.Error(w, "bad request", 400)
			return
		}
		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()
		messages, _ := req["messages"].([]any)
		toolResult := false
		needTool := false
		for _, raw := range messages {
			m := raw.(map[string]any)
			if m["role"] == "tool" {
				toolResult = true
			}
			if m["role"] == "user" && strings.Contains(fmt.Sprint(m["content"]), "USE_TOOL") {
				needTool = true
			}
		}
		delta := map[string]any{"role": "assistant", "content": "ACP_OK"}
		finish := "stop"
		if needTool && !toolResult {
			delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "tool-1", "type": "function", "function": map[string]any{"name": "searchTools", "arguments": `{"query":"readFile","intent":"Find the file reading tool"}`}}}}
			finish = "tool_calls"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, choice := range []any{map[string]any{"index": 0, "delta": delta, "finish_reason": nil}, map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}} {
			data, _ := json.Marshal(map[string]any{"id": "chatcmpl-test", "object": "chat.completion.chunk", "created": 1, "model": "test-model", "choices": []any{choice}})
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()
	home, cwd := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), []byte(`{"mcpServers":{}}`), 0600); err != nil {
		t.Fatal(err)
	}

	launch := func() *acpClient {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		cmd := exec.CommandContext(ctx, binary, "acp")
		cmd.Env = append(os.Environ(), "SOLOMON_HOME="+home, "SOLOMON_MCP_CONFIG="+filepath.Join(home, "mcp.json"), "OPENAI_BASE_URL="+provider.URL+"/v1", "OPENAI_API_KEY=test-key", "MODEL_ID=test-model")
		in, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		pin, pw := io.Pipe()
		pr, pout := io.Pipe()
		c := &acpClient{input: pw, output: pr, messages: make(chan map[string]any, 100), done: make(chan error, 1)}
		go func() { io.Copy(in, pin); in.Close() }()
		go func() {
			defer close(c.messages)
			defer pout.Close()
			dec := json.NewDecoder(out)
			for {
				var m map[string]any
				if err := dec.Decode(&m); err != nil {
					if err != io.EOF {
						t.Errorf("invalid protocol output: %v", err)
					}
					break
				}
				c.messages <- m
			}
			err := cmd.Wait()
			if err != nil {
				err = fmt.Errorf("%w: %s", err, stderr.String())
			}
			c.done <- err
			cancel()
		}()
		t.Cleanup(func() { c.stop(t) })
		return c
	}
	c := launch()
	c.init(t)
	session := c.call(t, "session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}})
	id := session["sessionId"].(string)
	for _, raw := range session["configOptions"].([]any) {
		option := raw.(map[string]any)
		if option["id"] != "reasoning" {
			continue
		}
		choices := option["options"].([]any)
		if len(choices) != 6 || strings.Contains(fmt.Sprint(choices), "default") {
			t.Fatalf("unexpected reasoning choices: %v", choices)
		}
		if option["currentValue"] != "medium" {
			t.Fatalf("unexpected initial reasoning: %v", option)
		}
	}
	if !strings.Contains(fmt.Sprint(session["configOptions"]), "ci-env/second-model") || !strings.Contains(fmt.Sprint(session["models"]), "ci-env/second-model") {
		t.Fatalf("provider model catalogue missing from session response: %v", session)
	}
	limited := c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/limited-model"})
	for _, raw := range limited["configOptions"].([]any) {
		option := raw.(map[string]any)
		if option["id"] == "reasoning" {
			if len(option["options"].([]any)) != 2 {
				t.Fatalf("wrong limited levels: %v", option)
			}
		}
	}
	rejected, _ := c.wait(t, c.request(t, "session/set_config_option", map[string]any{"sessionId": id, "configId": "reasoning", "value": "max"}))
	if rejected["error"] == nil {
		t.Fatal("unsupported reasoning accepted")
	}
	c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/test-model"})
	promptID := c.request(t, "session/prompt", promptParams(id, "USE_TOOL"))
	approved, seenTool, seenText := false, false, false
	var toolUpdates []any
	for {
		m := c.next(t)
		if m["method"] == "session/request_permission" {
			approved = true
			c.send(t, map[string]any{"jsonrpc": "2.0", "id": m["id"], "result": map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}})
		}
		if m["method"] == "session/update" {
			u := m["params"].(map[string]any)["update"].(map[string]any)
			if u["sessionUpdate"] == "tool_call_update" {
				toolUpdates = append(toolUpdates, u)
			}
			if u["sessionUpdate"] == "tool_call_update" && u["status"] == "completed" {
				seenTool = true
			}
			if u["sessionUpdate"] == "agent_message_chunk" {
				seenText = true
			}
		}
		if m["id"] == float64(promptID) {
			if m["error"] != nil {
				t.Fatalf("prompt failed: %v", m)
			}
			break
		}
	}
	if approved || !seenTool || !seenText {
		t.Fatalf("incomplete tool conversation: permission=%v tool=%v text=%v toolUpdates=%v", approved, seenTool, seenText, toolUpdates)
	}
	c.call(t, "session/set_config_option", map[string]any{"sessionId": id, "configId": "reasoning", "value": "low"})
	c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/second-model"})
	c.call(t, "session/set_config_option", map[string]any{"sessionId": id, "configId": "reasoning", "value": "high"})
	restored := c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/test-model"})
	if !strings.Contains(fmt.Sprint(restored), "currentValue:low") {
		t.Fatalf("model-specific reasoning not restored: %v", restored)
	}
	c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/second-model"})
	c.call(t, "session/set_config_option", map[string]any{"sessionId": id, "configId": "reasoning", "value": "default"})
	c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/test-model"})
	restored = c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/second-model"})
	if !strings.Contains(fmt.Sprint(restored), "currentValue:medium") {
		t.Fatal("unset reasoning did not resolve to medium")
	}
	c.call(t, "session/set_model", map[string]any{"sessionId": id, "modelId": "ci-env/test-model"})
	c.stop(t)
	c = launch()
	c.init(t)
	result := c.call(t, "session/resume", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": []any{}})
	if !strings.Contains(fmt.Sprint(result), "currentValue:low") {
		t.Fatalf("session config not restored: %v", result)
	}
	c.call(t, "session/prompt", map[string]any{"sessionId": id, "prompt": []any{
		map[string]any{"type": "text", "text": "Remember the preceding tool result."},
		map[string]any{"type": "image", "mimeType": "image/png", "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII="},
		map[string]any{"type": "resource", "resource": map[string]any{"uri": "file:///embedded.txt", "text": "embedded payload"}},
	}})
	mu.Lock()
	defer mu.Unlock()
	last := fmt.Sprint(requests[len(requests)-1]["messages"])
	if !strings.Contains(last, "image_url") || !strings.Contains(last, "embedded payload") {
		t.Fatal("multimodal content was not forwarded to the provider")
	}
	if !strings.Contains(last, "role:tool") || !strings.Contains(last, "tool_call_id:tool-1") {
		t.Fatal("tool result lost across process restart")
	}
}
