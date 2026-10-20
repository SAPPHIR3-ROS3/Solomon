package server

import (
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestSidebarOrderChangesOnSendOnly(t *testing.T) {
	created := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	first := &chatstore.Session{CreatedAt: created, LastUserMessageAt: created.Add(time.Hour)}
	second := &chatstore.Session{CreatedAt: created, LastUserMessageAt: created.Add(2 * time.Hour)}
	if !sidebarChatSentTime(second).After(sidebarChatSentTime(first)) {
		t.Fatal("the later user send must come first")
	}
	first.LastMessageAt = created.Add(3 * time.Hour)
	first.Messages = append(first.Messages, chatstore.Message{Role: "assistant", CreatedAt: first.LastMessageAt})
	if !sidebarChatSentTime(second).After(sidebarChatSentTime(first)) {
		t.Fatal("finishing an earlier response must not reorder chats")
	}
	firstProject := apiProject{Chats: []apiChatSummary{{LastMessageAt: sidebarChatSentTime(first)}}}
	secondProject := apiProject{Chats: []apiChatSummary{{LastMessageAt: sidebarChatSentTime(second)}}}
	if !projectLastActivity(secondProject).After(projectLastActivity(firstProject)) {
		t.Fatal("finishing an earlier response must not reorder folders")
	}
	first.LastUserMessageAt = created.Add(4 * time.Hour)
	if !sidebarChatSentTime(first).After(sidebarChatSentTime(second)) {
		t.Fatal("sending another message must promote the chat again")
	}
}

func TestSidebarLegacyChatUsesUserMessagesOnly(t *testing.T) {
	created := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	sess := &chatstore.Session{CreatedAt: created, LastMessageAt: created.Add(3 * time.Hour), Messages: []chatstore.Message{
		{Role: "user", CreatedAt: created.Add(time.Hour)},
		{Role: "assistant", CreatedAt: created.Add(3 * time.Hour)},
	}}
	if got := sidebarChatSentTime(sess); !got.Equal(created.Add(time.Hour)) {
		t.Fatalf("legacy send timestamp = %v", got)
	}
	sess.Messages = nil
	if got := sidebarChatSentTime(sess); !got.Equal(created) {
		t.Fatalf("empty chat timestamp = %v, want creation time", got)
	}
}
