package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_streamsFailedSDKToolLifecycle(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"intent": "inspect file", "source": `package main
import "sdk"
func main() { _, _ = sdk.ReadFile("missing.txt", "inspect missing file") }`})
	first := anthropicSSEBody(
		map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
		map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": "orchestration-live", "name": "orchestrate"}},
		map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(args)}},
		map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "tool_use"}},
	)
	second := anthropicSSEBody(
		map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
		map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "done"}},
		map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
	)
	var count atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if count.Add(1) == 1 {
			_, _ = io.WriteString(w, first)
		} else {
			_, _ = io.WriteString(w, second)
		}
	}))
	defer provider.Close()
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	config := fmt.Sprintf("[current]\nprovider = \"test\"\nmodel = \"claude-test\"\n[providers.test]\nbase_url = %q\napi_key = \"test-key\"\napi_protocol = \"anthropic\"\n", provider.URL)
	if err := os.WriteFile(filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	response := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()})
	var project struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, response, &project)
	response = postJSONForServerTest(t, server.URL+"/__solomon/projects/"+project.Project.ID+"/chats", map[string]string{})
	var chat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, response, &chat)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/__solomon/projects/"+project.Project.ID+"/chats/"+chat.ID+"/messages", bytes.NewBufferString(`{"content":"inspect file"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	response, err = (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	started, finished := false, false
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) != nil || event["name"] != "readFile" {
			continue
		}
		if event["parent_tool_call_id"] != "orchestration-live" || event["id"] != "orchestration-live/sdk/1" {
			t.Fatalf("incorrect child correlation: %v", event)
		}
		if event["type"] == "tool_start" {
			started = true
		}
		if event["type"] == "tool_result" {
			if !started || !strings.Contains(fmt.Sprint(event["error"]), "missing.txt") {
				t.Fatalf("missing start or failure: %v", event)
			}
			finished = true
		}
	}
	if !started || !finished {
		t.Fatalf("missing SDK lifecycle in stream:\n%s", body)
	}
}
