package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_firstChatGeneratesTitleConcurrentlyWithAssistant(t *testing.T) {
	sse := anthropicSSEBody(
		map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
		map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "answer"}},
		map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
	)
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var started sync.WaitGroup
	started.Add(2)
	release := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode provider request: %v", err)
			return
		}
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			previous := maxInFlight.Load()
			if current <= previous || maxInFlight.CompareAndSwap(previous, current) {
				break
			}
		}
		started.Done()
		<-release
		if stream, _ := request["stream"].(bool); stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Generated title"}},
		})
	}))
	defer provider.Close()

	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	writeChatProviderConfig(t, provider.URL)
	projectID, chatID := createServerTestChat(t, server.URL)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/__solomon/projects/"+projectID+"/chats/"+chatID+"/messages", bytes.NewBufferString("{\"content\":\"hello from the first message\"}"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	result := make(chan struct {
		response *http.Response
		err      error
	}, 1)
	go func() {
		response, requestErr := (&http.Client{Timeout: 8 * time.Second}).Do(request)
		result <- struct {
			response *http.Response
			err      error
		}{response: response, err: requestErr}
	}()

	waitStarted := make(chan struct{})
	go func() {
		started.Wait()
		close(waitStarted)
	}()
	select {
	case <-waitStarted:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("assistant and title requests did not overlap")
	}
	if maxInFlight.Load() < 2 {
		close(release)
		t.Fatalf("provider max in-flight = %d, want at least 2", maxInFlight.Load())
	}
	close(release)

	outcome := <-result
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	responseBody, err := io.ReadAll(outcome.response.Body)
	outcome.response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if outcome.response.StatusCode != http.StatusOK {
		t.Fatalf("send message status = %d body=%s", outcome.response.StatusCode, responseBody)
	}
	if !bytes.Contains(responseBody, []byte("\"type\":\"chat_snapshot\"")) || !bytes.Contains(responseBody, []byte("\"title\":\"generated-title\"")) {
		t.Fatalf("final snapshot does not contain generated title: %s", responseBody)
	}
}
