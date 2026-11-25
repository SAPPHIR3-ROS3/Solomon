package test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_runningChatKeepsRunStartAcrossFetches(t *testing.T) {
	providerStarted := make(chan struct{})
	releaseProvider := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseProvider) }) }
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !request.Stream {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"Generated title"}]}`)
			return
		}
		close(providerStarted)
		w.Header().Set("Content-Type", "text/event-stream")
		const stop = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
		_, _ = io.WriteString(w, strings.TrimSuffix(anthropicSSEBody(
			map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": "Plan"}},
		), stop))
		w.(http.Flusher).Flush()
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, strings.TrimSuffix(anthropicSSEBody(
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "ok"}},
		), stop))
		w.(http.Flusher).Flush()
		<-releaseProvider
		_, _ = io.WriteString(w, anthropicSSEBody(
			map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
		))
	}))
	defer provider.Close()

	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	defer release()
	writeChatProviderConfig(t, provider.URL)
	projectID, chatID := createServerTestChat(t, server.URL)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/__solomon/projects/"+projectID+"/chats/"+chatID+"/messages", bytes.NewBufferString(`{"content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	postDone := make(chan error, 1)
	deltaReceived := make(chan struct{})
	go func() {
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr != nil {
			postDone <- requestErr
			return
		}
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		var received bool
		for scanner.Scan() {
			data, ok := strings.CutPrefix(scanner.Text(), "data: ")
			if !ok || received {
				continue
			}
			var event struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &event) == nil && event.Type == "assistant_delta" && event.Delta == "ok" {
				received = true
				close(deltaReceived)
			}
		}
		postDone <- scanner.Err()
	}()

	select {
	case <-providerStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("provider request did not start")
	}
	first := fetchRunningChatTiming(t, server.URL, projectID, chatID)
	time.Sleep(20 * time.Millisecond)
	second := fetchRunningChatTiming(t, server.URL, projectID, chatID)
	if first.RunStartedAt == "" || second.RunStartedAt != first.RunStartedAt {
		t.Fatalf("run start changed across fetches: first=%q second=%q", first.RunStartedAt, second.RunStartedAt)
	}
	if _, err := time.Parse(time.RFC3339Nano, first.RunStartedAt); err != nil {
		t.Fatalf("invalid run start %q: %v", first.RunStartedAt, err)
	}

	select {
	case <-deltaReceived:
	case err := <-postDone:
		t.Fatalf("chat stream closed before the assistant delta: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("assistant delta was not streamed while the provider was running")
	}
	release()
	if err := <-postDone; err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(server.URL + "/__solomon/projects/" + projectID + "/chats/" + chatID)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Messages []struct {
			Content    string  `json:"content"`
			Reasoning  string  `json:"reasoning"`
			ThoughtFor float64 `json:"thoughtFor"`
		} `json:"messages"`
	}
	decodeServerTestJSON(t, response, &saved)
	for _, message := range saved.Messages {
		if message.Content == "ok" {
			if message.Reasoning != "Plan" || message.ThoughtFor < 0.02 {
				t.Fatalf("persisted reasoning = %+v, want Plan with its streamed duration", message)
			}
			return
		}
	}
	t.Fatal("assistant response is missing from the saved chat")
}

func writeChatProviderConfig(t *testing.T, providerURL string) {
	t.Helper()
	config := fmt.Sprintf("[current]\nprovider = \"test\"\nmodel = \"claude-test\"\n\n[providers.test]\nbase_url = %q\napi_key = \"test-key\"\napi_protocol = \"anthropic\"\n", providerURL)
	if err := os.WriteFile(filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

func createServerTestChat(t *testing.T, serverURL string) (string, string) {
	t.Helper()
	projectResponse := postJSONForServerTest(t, serverURL+"/__solomon/projects", map[string]string{"path": t.TempDir()})
	var project struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &project)
	chatResponse := postJSONForServerTest(t, serverURL+"/__solomon/projects/"+project.Project.ID+"/chats", map[string]string{})
	var chat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &chat)
	return project.Project.ID, chat.ID
}

func fetchRunningChatTiming(t *testing.T, serverURL, projectID, chatID string) struct {
	RunStartedAt string `json:"runStartedAt"`
	Status       string `json:"status"`
} {
	t.Helper()
	response, err := http.Get(serverURL + "/__solomon/projects/" + projectID + "/chats/" + chatID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var chat struct {
		RunStartedAt string `json:"runStartedAt"`
		Status       string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if chat.Status != "running" {
		t.Fatalf("chat status = %q, want running", chat.Status)
	}
	return chat
}
