package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestRateLimitResumeUsesProviderResetAndSavedSessions(t *testing.T) {
	requests := make(chan string, 8)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Model string }
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- request.Model
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, anthropicSSEBody(
			map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "resumed"}},
			map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
		))
	}))
	defer provider.Close()

	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	writeChatProviderConfig(t, provider.URL)
	projectID, chatID := createServerTestChat(t, server.URL)
	chatURL := server.URL + "/__solomon/projects/" + projectID + "/chats/" + chatID
	session, err := chatstore.ReadSession(projectID, chatID)
	if err != nil {
		t.Fatal(err)
	}
	session.Messages = []chatstore.Message{{Role: "user", Content: "original task"}}
	resetAt := time.Now().Add(2 * time.Second)
	notice := "summary: rate limit reached"
	session.RateLimitNotice = notice
	session.RateLimitResume = &chatstore.RateLimitResume{Provider: "test", Model: "claude-original", ResetAt: resetAt.Add(-time.Second), ResumeParent: true}
	if err := chatstore.WriteSession(projectID, session); err != nil {
		t.Fatal(err)
	}

	for _, child := range []*chatstore.SubSession{
		{ID: "child", RoleProvider: "test", RoleModel: "claude-child", Depth: 1, RateLimitNotice: notice},
		{ID: "grandchild", ParentSubchatID: "child", RoleProvider: "test", RoleModel: "claude-grandchild", Depth: 2, RateLimitNotice: notice, RateLimitResetAt: resetAt},
		{ID: "manual-pause", RoleProvider: "test", RoleModel: "claude-manually-paused"},
	} {
		child.Origin, child.ProjectHex, child.ParentChatID = chatstore.SubOriginParent, projectID, chatID
		child.Status = chatstore.SubStatusPaused
		child.Messages = []chatstore.Message{{Role: "user", Content: "original " + child.ID + " task"}}
		if err := chatstore.WriteSubSession(projectID, child); err != nil {
			t.Fatal(err)
		}
	}
	post := func() {
		t.Helper()
		response := postJSONForServerTest(t, chatURL+"/rate-limit-resume", map[string]string{})
		var result struct{ ScheduledAt time.Time }
		decodeServerTestJSON(t, response, &result)
		if response.StatusCode != http.StatusAccepted || !result.ScheduledAt.Equal(resetAt) {
			t.Fatalf("schedule status=%d reset=%s, want %s", response.StatusCode, result.ScheduledAt, resetAt)
		}
	}
	post()
	req, err := http.NewRequest(http.MethodDelete, chatURL+"/rate-limit-resume", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel status=%d", response.StatusCode)
	}
	post()
	post()

	response, err = http.Get(chatURL)
	if err != nil {
		t.Fatal(err)
	}
	var reopened struct {
		RateLimitScheduledAt time.Time
		Status               string
	}
	decodeServerTestJSON(t, response, &reopened)
	if !reopened.RateLimitScheduledAt.Equal(resetAt) || reopened.Status == "running" {
		t.Fatalf("reopened snapshot: %+v", reopened)
	}

	for _, want := range []string{"claude-grandchild", "claude-child", "claude-original"} {
		select {
		case model := <-requests:
			if model != want || time.Now().Before(resetAt) {
				t.Fatalf("unexpected request %q before reset or in place of %q", model, want)
			}
		case <-time.After(6 * time.Second):
			t.Fatalf("no request for %q after reset", want)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		session, err = chatstore.ReadSession(projectID, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if session.RateLimitNotice == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("resume did not clear the interruption")
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, message := range session.Messages {
		if message.Role == "user" && !message.GUIHidden && message.Content != "original task" {
			t.Fatalf("unexpected user message: %+v", message)
		}
	}
	manual, err := chatstore.FindSubSessionByID(projectID, "manual-pause")
	if err != nil || manual.Status != chatstore.SubStatusPaused {
		t.Fatalf("manually paused subagent changed: %+v (%v)", manual, err)
	}
}

func TestRateLimitDetailsUsesProviderTimestamp(t *testing.T) {
	observed := time.Unix(1770000000, 0)
	for _, tt := range []struct {
		body string
		want time.Time
	}{
		{`{"error":{"type":"usage_limit_reached","resets_at":1770000120,"resets_in_seconds":100}}`, observed.Add(2 * time.Minute)},
		{`{"error":{"type":"usage_limit_reached","resets_in_seconds":100}}`, observed.Add(100 * time.Second)},
	} {
		_, reset, ok := llm.RateLimitDetails(llm.NewProviderHTTPError(429, tt.body, 0), observed)
		if !ok || !reset.Equal(tt.want) {
			t.Fatalf("provider reset=%s, want %s", reset, tt.want)
		}
	}
}
