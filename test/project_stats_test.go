package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func TestProjectWelcomeStatsIncremental(t *testing.T) {
	stopCursorSidecar(t)
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	hex := "93619f1ceceeb7a95e04d2d628313536bbde0774ac260359b480be61e04b58d2"
	proot, err := paths.ProjectRoot(hex)
	if err != nil {
		t.Fatal(err)
	}
	chatsDir := filepath.Join(proot, "chats")
	if err := os.MkdirAll(chatsDir, 0o700); err != nil {
		t.Fatal(err)
	}

	s1 := &chatstore.Session{
		ID:        "chat-a",
		CreatedAt: time.Now(),
		Messages: []chatstore.Message{
			{Role: "assistant", UserPromptTokens: 10, ReasoningTokens: 2, ResponseTokens: 20},
		},
	}
	if err := chatstore.WriteSession(hex, s1); err != nil {
		t.Fatal(err)
	}
	n, u, r, resp, err := chatstore.ProjectWelcomeStats(hex)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || u != 10 || r != 2 || resp != 20 {
		t.Fatalf("after first write: chats=%d u=%d r=%d resp=%d", n, u, r, resp)
	}

	s2 := &chatstore.Session{
		ID:        "chat-b",
		CreatedAt: time.Now(),
		Messages: []chatstore.Message{
			{Role: "assistant", UserPromptTokens: 5, ReasoningTokens: 1, ResponseTokens: 4},
		},
	}
	if err := chatstore.WriteSession(hex, s2); err != nil {
		t.Fatal(err)
	}
	n, u, r, resp, err = chatstore.ProjectWelcomeStats(hex)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || u != 15 || r != 3 || resp != 24 {
		t.Fatalf("after second write: chats=%d u=%d r=%d resp=%d", n, u, r, resp)
	}

	if err := chatstore.RemoveSessionPath(hex, "chat-a"); err != nil {
		t.Fatal(err)
	}
	n, u, r, resp, err = chatstore.ProjectWelcomeStats(hex)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || u != 5 {
		t.Fatalf("after remove: chats=%d u=%d", n, u)
	}
}

func TestListRecentOrdersByLatestActivity(t *testing.T) {
	stopCursorSidecar(t)
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	hex := "93619f1ceceeb7a95e04d2d628313536bbde0774ac260359b480be61e04b58d2"
	proot, err := paths.ProjectRoot(hex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proot, "chats"), 0o700); err != nil {
		t.Fatal(err)
	}
	older := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)
	createdFirst := &chatstore.Session{
		ID:            "created-first",
		CreatedAt:     older,
		LastMessageAt: older,
		Messages:      []chatstore.Message{{CreatedAt: newer, Role: "user", Content: "later"}},
	}
	createdLater := &chatstore.Session{
		ID:            "created-later",
		CreatedAt:     newer,
		LastMessageAt: newer,
	}
	if err := chatstore.WriteSession(hex, createdLater); err != nil {
		t.Fatal(err)
	}
	if err := chatstore.WriteSession(hex, createdFirst); err != nil {
		t.Fatal(err)
	}
	list, err := chatstore.ListRecent(hex, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "created-first" {
		ids := make([]string, 0, len(list))
		for _, sess := range list {
			ids = append(ids, sess.ID)
		}
		t.Fatalf("ListRecent order = %v, want created-first first", ids)
	}
}
