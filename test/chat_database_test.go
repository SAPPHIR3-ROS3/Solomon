package test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

func TestDatabaseChatPersistsOrderedStreamsAndAttachments(t *testing.T) {
	home := t.TempDir()
	setSolomonHome(t, home)
	const project = "project"
	const id = "original"
	now := time.Unix(1700000000, 321).UTC()
	first := chatstore.Message{
		Role: "assistant", Content: "first", CreatedAt: now,
		ToolImages: []images.Attachment{{Path: "source.png", Data: []byte("snapshot")}},
	}
	second := chatstore.Message{Role: "assistant", Content: "second", CreatedAt: now}
	session := &chatstore.Session{
		ID: id, Title: "Example", CreatedAt: now, CheckpointCP0: true,
		Messages: []chatstore.Message{second, first, first},
		Branches: []chatstore.BranchSegment{{ForkAtInclusive: 0, Messages: []chatstore.Message{first, second}}},
		UncompactedRaw: []chatstore.UncompactedDump{{
			CompactAt: now, Messages: []chatstore.Message{second, first},
			Branches: []chatstore.BranchSegment{{ForkAtInclusive: 1, Messages: []chatstore.Message{second}}},
		}},
	}
	if err := chatstore.WriteSession(project, session); err != nil {
		t.Fatal(err)
	}
	path, err := chatstore.SessionPath(project, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new data used the legacy file: %v", err)
	}
	db := openTestDatabase(t, home, database.Data)
	var messages, positions int
	if err := db.QueryRow("SELECT (SELECT count(*) FROM chat_message), (SELECT count(*) FROM chat_membership)").Scan(&messages, &positions); err != nil {
		t.Fatal(err)
	}
	if messages != 2 || positions != 8 {
		t.Fatalf("stored %d unique messages with %d positions", messages, positions)
	}
	reopened, err := chatstore.ReadSession(project, id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reopened.Messages, session.Messages) || !reflect.DeepEqual(reopened.Branches, session.Branches) || !reflect.DeepEqual(reopened.UncompactedRaw, session.UncompactedRaw) {
		t.Fatal("stream order, branches, compaction or attachment bytes changed")
	}
	if err := chatstore.RenameSessionFile(project, id, "renamed"); err != nil {
		t.Fatal(err)
	}
	alias, err := chatstore.ReadSession(project, id)
	if err != nil || alias.ID != "renamed" {
		t.Fatalf("legacy alias got %v, %v", alias, err)
	}
	list, err := chatstore.ListSummaries(project, -1)
	if err != nil || len(list) != 1 || list[0].ID != "renamed" {
		t.Fatalf("list got %v, %v", list, err)
	}
	if err := chatstore.RemoveSessionPath(project, "renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := chatstore.ReadSession(project, id); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted alias still resolves: %v", err)
	}
}

func TestDatabaseChatImportsOnceAndKeepsSource(t *testing.T) {
	setSolomonHome(t, t.TempDir())
	path, err := chatstore.SessionPath("project", "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(chatstore.Session{ID: "legacy", Title: "before"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	session, err := chatstore.ReadSession("project", "legacy")
	if err != nil {
		t.Fatal(err)
	}
	session.Title = "after"
	if err := chatstore.WriteSession("project", session); err != nil {
		t.Fatal(err)
	}
	if retained, err := os.ReadFile(path); err != nil || string(retained) != string(data) {
		t.Fatalf("legacy source changed: %v", err)
	}
	reopened, err := chatstore.ReadSession("project", "legacy")
	if err != nil || reopened.Title != "after" {
		t.Fatalf("stored update did not win: %v, %v", reopened, err)
	}
}

func TestDatabaseSubagentPreservesResumeState(t *testing.T) {
	setSolomonHome(t, t.TempDir())
	at := time.Unix(1700000000, 123).UTC()
	agent := &chatstore.SubSession{
		ID: "test-agent", ProjectHex: "project", Origin: chatstore.SubOriginScheduled,
		Status: chatstore.SubStatusPaused, CreatedAt: at,
		Messages:      []chatstore.Message{{Role: "assistant", Content: "previous work"}},
		TaskQueue:     []chatstore.QueuedTask{{Task: "first", EnqueuedAt: at}, {Task: "second", EnqueuedAt: at}},
		PendingSpawns: []chatstore.PendingSubagentSpawn{{Task: "nested", ParentSubchatID: "test-agent", CreatedAt: at}},
	}
	if err := chatstore.WriteSubSession("project", agent); err != nil {
		t.Fatal(err)
	}
	loaded, err := chatstore.ReadSubSession("", chatstore.SubOriginScheduled, "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(agent)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(loaded)
	if err != nil || string(expected) != string(actual) {
		t.Fatalf("resume state changed: %s, %v", actual, err)
	}
	list, err := chatstore.ListSubSessions("project")
	if err != nil || len(list) != 1 || list[0].ID != agent.ID {
		t.Fatalf("scheduled agent list got %v, %v", list, err)
	}
}

func TestDatabaseChatUpdatesOnlyChangedPositions(t *testing.T) {
	home := t.TempDir()
	setSolomonHome(t, home)
	now := time.Unix(1700000000, 0).UTC()
	first := chatstore.Message{Role: "user", Content: "first", CreatedAt: now}
	second := chatstore.Message{Role: "assistant", Content: "second", CreatedAt: now}
	session := &chatstore.Session{
		ID: "incremental", CreatedAt: now, CheckpointCP0: true,
		Messages: []chatstore.Message{first, second},
		Branches: []chatstore.BranchSegment{{Messages: []chatstore.Message{second}}},
	}
	if err := chatstore.WriteSession("project", session); err != nil {
		t.Fatal(err)
	}
	db := openTestDatabase(t, home, database.Data)
	if _, err := db.Exec(`
CREATE TABLE test_position_writes (operation TEXT);
CREATE TRIGGER test_insert AFTER INSERT ON chat_membership BEGIN INSERT INTO test_position_writes VALUES ('insert'); END;
CREATE TRIGGER test_update AFTER UPDATE ON chat_membership BEGIN INSERT INTO test_position_writes VALUES ('update'); END;
CREATE TRIGGER test_delete AFTER DELETE ON chat_membership BEGIN INSERT INTO test_position_writes VALUES ('delete'); END;
`); err != nil {
		t.Fatal(err)
	}
	assertWrite := func(operation string, mutate func()) {
		t.Helper()
		mutate()
		if err := chatstore.WriteSession("project", session); err != nil {
			t.Fatal(err)
		}
		var writes string
		if err := db.QueryRow("SELECT COALESCE(group_concat(operation), '') FROM test_position_writes").Scan(&writes); err != nil || writes != operation {
			t.Fatalf("writes = %q, %v, want %q", writes, err, operation)
		}
		if _, err := db.Exec("DELETE FROM test_position_writes"); err != nil {
			t.Fatal(err)
		}
	}
	assertWrite("", func() { session.Title = "Metadata only" })
	assertWrite("insert", func() { session.Messages = append(session.Messages, first) })
	assertWrite("update", func() { session.Messages[1].Content = "edited" })
	assertWrite("delete,delete", func() {
		session.Messages = session.Messages[:2]
		session.Branches = nil
	})
	loaded, err := chatstore.ReadSession("project", session.ID)
	if err != nil || !reflect.DeepEqual(loaded.Messages, session.Messages) || len(loaded.Branches) != 0 {
		t.Fatalf("incorrect round trip: %v", err)
	}
	var messages int
	if err := db.QueryRow("SELECT count(*) FROM chat_message").Scan(&messages); err != nil || messages != 2 {
		t.Fatalf("orphan cleanup left %d messages, %v", messages, err)
	}
}

func TestDatabaseStatsCombinesLegacyAndStoredWithoutDuplicates(t *testing.T) {
	home := t.TempDir()
	setSolomonHome(t, home)
	const project = "project"
	legacy := &chatstore.Session{ID: "legacy", Messages: []chatstore.Message{{Role: "assistant", UserPromptTokens: 7}}}
	stored := &chatstore.Session{ID: "stored", Messages: []chatstore.Message{{Role: "assistant", UserPromptTokens: 11}}}
	path, err := chatstore.SessionPath(project, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := chatstore.WriteSession(project, stored); err != nil {
		t.Fatal(err)
	}
	db := openTestDatabase(t, home, database.Data)
	if _, err := db.Exec("DELETE FROM chat_usage"); err != nil {
		t.Fatal(err)
	}
	check := func(wantCount int, wantTokens int64) {
		t.Helper()
		count, tokens, _, _, err := chatstore.ProjectWelcomeStats(project)
		if err != nil || count != wantCount || tokens != wantTokens {
			t.Fatalf("stats = %d chats, %d tokens, %v; want %d chats, %d tokens", count, tokens, err, wantCount, wantTokens)
		}
	}
	check(2, 18)
	if _, err := chatstore.ReadSession(project, legacy.ID); err != nil {
		t.Fatal(err)
	}
	check(2, 18)
	if err := chatstore.RenameSessionFile(project, legacy.ID, "renamed"); err != nil {
		t.Fatal(err)
	}
	check(2, 18)
	if err := chatstore.RemoveSessionPath(project, "renamed"); err != nil {
		t.Fatal(err)
	}
	check(1, 11)
}

func TestDatabaseDirectMessageChangePreservesOtherPositions(t *testing.T) {
	home := t.TempDir()
	setSolomonHome(t, home)
	first := chatstore.Message{Role: "user", Content: "first"}
	last := chatstore.Message{Role: "assistant", Content: "last", ResponseTokens: 3}
	session := &chatstore.Session{
		ID: "direct", CheckpointCP0: true, Messages: []chatstore.Message{first, last},
		Branches: []chatstore.BranchSegment{{Messages: []chatstore.Message{first}}},
	}
	if err := chatstore.WriteSession("project", session); err != nil {
		t.Fatal(err)
	}
	db := openTestDatabase(t, home, database.Data)
	if _, err := db.Exec(`
CREATE TRIGGER test_direct_update BEFORE UPDATE ON chat_membership
WHEN OLD.stream != 'main' OR OLD.position != 0
BEGIN SELECT RAISE(ABORT, 'unrelated position updated'); END;
CREATE TRIGGER test_direct_delete BEFORE DELETE ON chat_membership
BEGIN SELECT RAISE(ABORT, 'unrelated position removed'); END;
`); err != nil {
		t.Fatal(err)
	}
	session.Messages[0].Content = "edited"
	if err := chatstore.WriteMainMessageChange("project", session, 2, 0); err != nil {
		t.Fatal(err)
	}
	session.Messages = append(session.Messages, last)
	if err := chatstore.WriteMainMessageChange("project", session, 2, 2); err != nil {
		t.Fatal(err)
	}
	stored, err := chatstore.ReadSession("project", session.ID)
	if err != nil || !reflect.DeepEqual(stored.Messages, session.Messages) || !reflect.DeepEqual(stored.Branches, session.Branches) {
		t.Fatalf("direct write did not preserve chat and branch: %v", err)
	}
	count, _, _, response, err := chatstore.ProjectWelcomeStats("project")
	if err != nil || count != 1 || response != 6 {
		t.Fatalf("stats = %d chats, %d response tokens, %v", count, response, err)
	}
}

func TestDatabaseSidebarMergesBeforeLimiting(t *testing.T) {
	home := t.TempDir()
	setSolomonHome(t, home)
	const project = "sidebar"
	base := time.Unix(1700000000, 0).UTC()
	makeSession := func(id string, sent time.Time) *chatstore.Session {
		return &chatstore.Session{
			ID: id, CreatedAt: base.Add(-time.Hour), LastUserMessageAt: sent,
			LastMessageAt: base.Add(time.Hour),
		}
	}
	for _, session := range []*chatstore.Session{
		makeSession("whole-second", base),
		makeSession("fractional", base.Add(time.Nanosecond)),
		makeSession("older", base.Add(-time.Second)),
	} {
		if err := chatstore.WriteSession(project, session); err != nil {
			t.Fatal(err)
		}
	}
	legacy := makeSession("legacy", base.Add(2*time.Nanosecond))
	path, err := chatstore.SessionPath(project, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := chatstore.ListSidebarSummaries(project, 2)
	if err != nil || len(got) != 2 || got[0].ID != legacy.ID || got[1].ID != "fractional" {
		t.Fatalf("limited sidebar = %v, %v", got, err)
	}
	if err := chatstore.WriteSession(project, legacy); err != nil {
		t.Fatal(err)
	}
	got, err = chatstore.ListSidebarSummaries(project, 2)
	if err != nil || len(got) != 2 || got[0].ID != legacy.ID || got[1].ID != "fractional" {
		t.Fatalf("imported legacy duplicated or reordered: %v, %v", got, err)
	}
}
