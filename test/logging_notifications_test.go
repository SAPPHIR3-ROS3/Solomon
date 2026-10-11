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
