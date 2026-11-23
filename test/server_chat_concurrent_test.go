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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_concurrentChatsAcrossProjects(t *testing.T) {
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
	var inFlight atomic.Int32
	bothStarted := make(chan struct{})
	var signalBoth sync.Once
	release := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		if current >= 2 {
			signalBoth.Do(func() { close(bothStarted) })
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse)
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

	type chatTarget struct {
		projectID string
		chatID    string
	}
	targets := make([]chatTarget, 0, 2)
	for i := 0; i < 2; i++ {
		projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()})
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
		targets = append(targets, chatTarget{projectID: created.Project.ID, chatID: createdChat.ID})
	}

	type streamResult struct {
		status int
		body   []byte
		err    error
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results := make([]streamResult, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(index int, target chatTarget) {
			defer wg.Done()
			body, err := json.Marshal(map[string]string{"content": fmt.Sprintf("hello-%d", index)})
			if err != nil {
				results[index] = streamResult{err: err}
				return
			}
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/__solomon/projects/"+target.projectID+"/chats/"+target.chatID+"/messages", bytes.NewReader(body))
			if err != nil {
				results[index] = streamResult{err: err}
				return
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "text/event-stream")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				results[index] = streamResult{err: err}
				return
			}
			responseBody, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			results[index] = streamResult{status: response.StatusCode, body: responseBody, err: readErr}
		}(i, target)
	}

	overlapped := false
	select {
	case <-bothStarted:
		overlapped = true
	case <-ctx.Done():
	}
	close(release)
	wg.Wait()
	if !overlapped {
		t.Fatalf("provider did not receive two overlapping chat requests: %+v", results)
	}

	for i, result := range results {
		if result.err != nil {
			t.Fatalf("chat %d stream error: %v", i, result.err)
		}
		if result.status != http.StatusOK {
			t.Fatalf("chat %d status = %d body=%s", i, result.status, result.body)
		}
		if !bytes.Contains(result.body, []byte(`"type":"chat_snapshot"`)) {
			t.Fatalf("chat %d missing snapshot: %s", i, result.body)
		}
	}
	getHealthForTest(t, server.URL)
}
