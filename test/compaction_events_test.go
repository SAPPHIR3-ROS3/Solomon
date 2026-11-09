package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestChatRunCompactionPublishesLiveSummarySnapshot(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	sess := &chatstore.Session{ID: "compacted-chat", Messages: []chatstore.Message{
		{Role: "user", Content: "earlier question"},
		{Role: "assistant", Content: "earlier answer"},
	}}
	body := "---\n[Conversation summary]\n---\n\nContinue the prototype.\n---"
	pending := chatstore.Message{Role: "user", Content: "next question"}
	chatstore.ApplyCompactionWithPendingMessages(sess, body, time.Now(), []chatstore.Message{pending})
	if err := chatstore.WriteSession(testProjectHex, sess); err != nil {
		t.Fatal(err)
	}
	events := serverruntime.ChatRunEventsForTest(
		cievents.ContextCompacted(testProjectHex, sess.ID),
		cievents.AssistantStart(0, 0),
	)
	if len(events) != 2 || events[0]["type"] != "chat_snapshot" || events[1]["type"] != cievents.TypeAssistantStart {
		t.Fatalf("events=%+v", events)
	}
	encoded, err := json.Marshal(events[0])
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Chat struct {
			Status   string `json:"status"`
			Messages []struct {
				Kind    string `json:"kind"`
				Summary string `json:"summary"`
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"chat"`
	}
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Chat.Status != "running" || len(snapshot.Chat.Messages) != 2 {
		t.Fatalf("snapshot=%s", encoded)
	}
	if snapshot.Chat.Messages[0].Kind != "compaction" || snapshot.Chat.Messages[0].Summary != "Continue the prototype." || snapshot.Chat.Messages[1].Content != pending.Content {
		t.Fatalf("summary or pending message missing: %s", encoded)
	}
}
