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
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerRuntime_chatSnapshotDoesNotResurrectRejectedIntentCall(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)

	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	var createdChat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	sess, err := chatstore.ReadSession(created.Project.ID, createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}
	sess.Messages = []chatstore.Message{{
		Role: "assistant",
		ToolCalls: []chatstore.ToolCall{{
			ID:        "rejected-1",
			Name:      "subagent",
			Arguments: `{"task":"inspect"}`,
		}},
	}}
	if err := chatstore.WriteSession(created.Project.ID, sess); err != nil {
		t.Fatal(err)
	}

	response, err := http.Get(server.URL + "/__solomon/projects/" + created.Project.ID + "/chats/" + createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("open chat status = %d", response.StatusCode)
	}
	var opened struct {
		Messages []struct {
			ToolCalls []struct {
				Result map[string]any `json:"result"`
				Status string         `json:"status"`
			} `json:"toolCalls"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&opened); err != nil {
		t.Fatal(err)
	}
	if len(opened.Messages) != 1 || len(opened.Messages[0].ToolCalls) != 1 {
		t.Fatalf("opened chat=%+v", opened)
	}
	tool := opened.Messages[0].ToolCalls[0]
	if tool.Status != "error" || tool.Result["status"] != "error" {
		t.Fatalf("rejected tool was resurrected: %+v", tool)
	}
	if errorText, _ := tool.Result["error"].(string); !strings.Contains(errorText, "missing tool intent") {
		t.Fatalf("unexpected rejection: %+v", tool.Result)
	}
}

func TestServerRuntime_chatProjectAndChatLifecycle(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	projectRoot := t.TempDir()

	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)
	if created.Project.ID == "" {
		t.Fatal("created project has no id")
	}

	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	if chatResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	var createdChat struct {
		ID        string `json:"id"`
		ProjectID string `json:"projectID"`
		Title     string `json:"title"`
		Messages  []any  `json:"messages"`
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)
	if createdChat.ID == "" || createdChat.ProjectID != created.Project.ID || createdChat.Title != "New chat" || len(createdChat.Messages) != 0 {
		t.Fatalf("created chat is incomplete: %+v", createdChat)
	}

	response, err := http.Get(server.URL + "/__solomon/projects/" + created.Project.ID + "/chats/" + createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("open chat status = %d", response.StatusCode)
	}
	var openedChat struct {
		ID       string `json:"id"`
		Messages []any  `json:"messages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&openedChat); err != nil {
		t.Fatalf("decode opened chat: %v", err)
	}
	if openedChat.ID != createdChat.ID || len(openedChat.Messages) != 0 {
		t.Fatalf("opened chat does not match created chat: %+v", openedChat)
	}

	response, err = http.Get(server.URL + "/__solomon/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("sidebar status = %d", response.StatusCode)
	}
	var sidebar struct {
		Projects []struct {
			ID    string `json:"id"`
			Chats []struct {
				ID string `json:"id"`
			} `json:"chats"`
		} `json:"projects"`
	}
	if err := json.NewDecoder(response.Body).Decode(&sidebar); err != nil {
		t.Fatalf("decode sidebar: %v", err)
	}
	if len(sidebar.Projects) != 1 || sidebar.Projects[0].ID != created.Project.ID || len(sidebar.Projects[0].Chats) != 1 || sidebar.Projects[0].Chats[0].ID != createdChat.ID {
		t.Fatalf("sidebar does not contain the created chat: %+v", sidebar.Projects)
	}
}

func TestServerRuntime_projectAtMentionSuggestionsUseCanonicalIndex(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)

	response, err := http.Get(server.URL + "/__solomon/projects/" + created.Project.ID + "/at-mentions?query=main")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("at-mention status = %d", response.StatusCode)
	}
	var suggestions []struct {
		Path string `json:"path"`
		Tag  string `json:"tag"`
	}
	if err := json.NewDecoder(response.Body).Decode(&suggestions); err != nil {
		t.Fatalf("decode at-mentions: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Path != "main.go" || suggestions[0].Tag != "@main.go" {
		t.Fatalf("unexpected at-mention suggestions: %+v", suggestions)
	}
}

func TestServerRuntime_sendChatMessageKeepsServerAlive(t *testing.T) {
	sse := anthropicSSEBody(
		map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"usage": map[string]any{"input_tokens": 1},
			},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "ok"},
		},
		map[string]any{
			"type":  "message_delta",
			"usage": map[string]any{"output_tokens": 1},
			"delta": map[string]any{"stop_reason": "end_turn"},
		},
	)
	provider := newAnthropicMockServer(t, "/v1/messages", nil, sse, 0)
	defer provider.Close()

	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	home := os.Getenv("SOLOMON_HOME")
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(fmt.Sprintf(`[current]
provider = "test"
model = "claude-test"

[providers.test]
base_url = %q
api_key = "test-key"
api_protocol = "anthropic"
`, provider.URL)), 0o600); err != nil {
		t.Fatal(err)
	}

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)

	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	var createdChat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	body, err := json.Marshal(map[string]string{"content": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats/"+createdChat.ID+"/messages", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("send message status = %d body=%s", response.StatusCode, responseBody)
	}
	if !bytes.Contains(responseBody, []byte(`"type":"chat_snapshot"`)) {
		t.Fatalf("send message response has no final snapshot: %s", responseBody)
	}

	getHealthForTest(t, server.URL)
}

func TestServerRuntime_sendChatMessageWithMCPKeepsServerAlive(t *testing.T) {
	sse := anthropicSSEBody(
		map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"usage": map[string]any{"input_tokens": 1},
			},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "ok"},
		},
		map[string]any{
			"type":  "message_delta",
			"usage": map[string]any{"output_tokens": 1},
			"delta": map[string]any{"stop_reason": "end_turn"},
		},
	)
	provider := newAnthropicMockServer(t, "/v1/messages", nil, sse, 0)
	defer provider.Close()

	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-mcp", Version: "1"}, nil)
	sdkmcp.AddTool(mcpServer, &sdkmcp.Tool{Name: "ping", Description: "ping"}, func(context.Context, *sdkmcp.CallToolRequest, map[string]any) (*sdkmcp.CallToolResult, struct{}, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "pong"}}}, struct{}{}, nil
	})
	mcpHTTP := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return mcpServer
	}, nil))
	defer mcpHTTP.Close()

	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	home := os.Getenv("SOLOMON_HOME")
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(fmt.Sprintf(`[current]
provider = "test"
model = "claude-test"

[providers.test]
base_url = %q
api_key = "test-key"
api_protocol = "anthropic"
`, provider.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), []byte(fmt.Sprintf(`{"mcpServers":{"test":{"type":"streamable-http","url":%q}}}`, mcpHTTP.URL)), 0o600); err != nil {
		t.Fatal(err)
	}

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)

	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	var createdChat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	body := bytes.NewBufferString(`{"content":"hello"}`)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats/"+createdChat.ID+"/messages", body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("send message status = %d body=%s", response.StatusCode, responseBody)
	}
	getHealthForTest(t, server.URL)
}

func TestServerRuntime_sendChatMessageWithNativeToolKeepsServerAlive(t *testing.T) {
	first := anthropicSSEBody(
		map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"usage": map[string]any{"input_tokens": 1},
			},
		},
		map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type": "tool_use",
				"id":   "toolu_list",
				"name": "listSubAgents",
			},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": `{}`},
		},
		map[string]any{
			"type":  "message_delta",
			"usage": map[string]any{"output_tokens": 1},
			"delta": map[string]any{"stop_reason": "tool_use"},
		},
	)
	second := anthropicSSEBody(
		map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"usage": map[string]any{"input_tokens": 1},
			},
		},
		map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "done"},
		},
		map[string]any{
			"type":  "message_delta",
			"usage": map[string]any{"output_tokens": 1},
			"delta": map[string]any{"stop_reason": "end_turn"},
		},
	)
	var providerCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if providerCalls.Add(1) == 1 {
			_, _ = io.WriteString(w, first)
			return
		}
		_, _ = io.WriteString(w, second)
	}))
	defer provider.Close()

	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	home := os.Getenv("SOLOMON_HOME")
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(fmt.Sprintf(`[current]
provider = "test"
model = "claude-test"

[providers.test]
base_url = %q
api_key = "test-key"
api_protocol = "anthropic"
`, provider.URL)), 0o600); err != nil {
		t.Fatal(err)
	}

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)

	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	var createdChat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats/"+createdChat.ID+"/messages", bytes.NewBufferString(`{"content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("send message status = %d body=%s", response.StatusCode, responseBody)
	}
	if providerCalls.Load() < 2 {
		t.Fatalf("provider calls = %d, want at least 2", providerCalls.Load())
	}
	if !bytes.Contains(responseBody, []byte(`"type":"tool_result"`)) || !bytes.Contains(responseBody, []byte(`"id":"toolu_list"`)) || !bytes.Contains(responseBody, []byte(`missing tool intent`)) {
		t.Fatalf("rejected native tool call did not get a terminal SSE result: %s", responseBody)
	}
	getHealthForTest(t, server.URL)
}

func postJSONForServerTest(t *testing.T, url string, value any) *http.Response {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeServerTestJSON(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
