package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func TestNotificationHistorySurvivesRestartAndIncludesOlderLogs(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(logging.IsolateNotificationsForTest(dir))
	legacy := "[2025-01-01T10:00:00Z][ERROR][test.go][test] Old failure\nsecond line\n[2025-01-01T11:00:00Z][INFO][test.go][test] Ordinary log\n"
	if err := os.WriteFile(filepath.Join(dir, "2025-01-01.log"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	logging.RecordNotification("system", "Saved configuration\nSuccessfully")
	logging.ClearNotificationMemoryForTest()
	entries, err := logging.ReadNotifications()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected both journal and old log, got %#v", entries)
	}
	if entries[0].Message != "Saved configuration\nSuccessfully" || entries[1].Message != "Old failure\nsecond line" {
		t.Fatalf("unexpected order or messages: %#v", entries)
	}
	again, err := logging.ReadNotifications()
	if err != nil || again[1].ID != entries[1].ID {
		t.Fatal("history IDs must remain stable across reads")
	}
}

func TestLoggedErrorIsNotDuplicatedByJournalAndFile(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(logging.IsolateNotificationsForTest(dir))
	logging.RecordNotificationAtForTest("2025-01-01T10:00:00Z", "error", "Failure")
	if err := os.WriteFile(filepath.Join(dir, "2025-01-01.log"), []byte("[2025-01-01T10:00:00Z][ERROR][test.go][test] Failure\n"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := logging.ReadNotifications()
	if err != nil || len(entries) != 1 {
		t.Fatalf("duplicate history: %#v, %v", entries, err)
	}
}

func TestNotificationPreservesBracketedContinuationLines(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(logging.IsolateNotificationsForTest(dir))
	message := "Failure\n[compiler output]\nmore details"
	logging.RecordNotificationAtForTest("2025-01-01T10:00:00Z", "error", message)
	legacy := "[2025-01-01T10:00:00Z][ERROR][test.go][test] " + message + "\n[2025-01-01T11:00:00Z][INFO][test.go][test] Done\n"
	if err := os.WriteFile(filepath.Join(dir, "2025-01-01.log"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := logging.ReadNotifications()
	if err != nil || len(entries) != 1 || entries[0].Message != message {
		t.Fatalf("truncated or duplicated notification: %#v, %v", entries, err)
	}
}

func TestNotificationsExcludeSystemGoodbye(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(logging.IsolateNotificationsForTest(dir))
	logging.RecordNotification("system", "Goodbye.\nResume this chat by id: /resume abc")
	if _, err := os.Stat(filepath.Join(dir, "notifications.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("goodbye should not be persisted, stat error: %v", err)
	}
	legacy := "[2025-01-01T10:00:00Z][SYSTEM][test.go][test] Goodbye.\nResume this chat by id: /resume abc\n"
	if err := os.WriteFile(filepath.Join(dir, "2025-01-01.log"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	journal := `{"id":"old-goodbye","createdAt":"2025-01-01T10:00:00Z","level":"system","message":"Goodbye.\nResume this chat by id: /resume abc"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "notifications.jsonl"), []byte(journal), 0600); err != nil {
		t.Fatal(err)
	}
	logging.RecordNotification("system", "Saved configuration")
	logging.RecordNotification("error", "Goodbye.")
	entries, err := logging.ReadNotifications()
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected notifications: %#v, %v", entries, err)
	}
	for _, entry := range entries {
		if entry.Level == "system" && entry.Message != "Saved configuration" {
			t.Fatalf("unexpected system notification: %#v", entry)
		}
	}
}
