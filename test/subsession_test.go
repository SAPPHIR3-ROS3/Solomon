package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func TestSubchatIDUniqueWithSpawnTime(t *testing.T) {
	tc := chatstore.ToolCall{ID: "call_1", Name: "subagent", Arguments: `{"task":"x"}`}
	t1 := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Second)
	id1 := chatstore.SubchatID("parent", tc, t1)
	id2 := chatstore.SubchatID("parent", tc, t2)
	if id1 == id2 {
		t.Fatal("expected different ids for different spawn times")
	}
	if id1 == "" || id2 == "" {
		t.Fatal("expected non-empty ids")
	}
}

func TestSubSessionWriteReadParent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	projHex := "abc123"
	if err := os.MkdirAll(filepath.Join(home, "projects", projHex, "chats", "subchats"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s := &chatstore.SubSession{
		ID:            "sub1",
		Title:         "test-sub",
		CreatedAt:     now,
		LastMessageAt: now,
		Origin:        chatstore.SubOriginParent,
		ProjectHex:    projHex,
		Status:        chatstore.SubStatusDone,
		Messages:      []chatstore.Message{{Role: "user", Content: "hello"}},
	}
	if err := chatstore.WriteSubSession(projHex, s); err != nil {
		t.Fatal(err)
	}
	got, err := chatstore.ReadSubSession(projHex, chatstore.SubOriginParent, "sub1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "test-sub" || len(got.Messages) != 1 {
		t.Fatalf("unexpected session: %+v", got)
	}
}

func TestActiveSubagentsRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := paths.EnsureSubagentsDir(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	f := &chatstore.ActiveSubagentsFile{
		Agents: []chatstore.ActiveSubagentEntry{{
			ID: "id1", Origin: chatstore.SubOriginParent, Status: chatstore.SubStatusRunning, SpawnedAt: now,
		}},
	}
	if err := chatstore.WriteActiveSubagents(f); err != nil {
		t.Fatal(err)
	}
	got, err := chatstore.ReadActiveSubagents()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Agents) != 1 || got.Agents[0].ID != "id1" {
		t.Fatalf("unexpected: %+v", got)
	}
}
