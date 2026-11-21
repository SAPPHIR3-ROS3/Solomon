package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
)

func TestDatabaseMigrationRetriesBusyWrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	path, err := chatstore.SessionPath("legacy", "legacy-chat")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(chatstore.Session{ID: "legacy-chat"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	lease, err := database.BorrowData(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	blocker, err := lease.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		chatstore.RunChatMigration(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()
	timer := time.NewTimer(3500 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	var imported, failed int
	if err := blocker.QueryRow(`
SELECT (SELECT count(*) FROM chat_session), (SELECT count(*) FROM chat_import_failure)`).Scan(&imported, &failed); err != nil {
		t.Fatal(err)
	}
	if imported != 0 || failed != 0 {
		t.Fatalf("while locked: %d imported, %d recorded failures", imported, failed)
	}
	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("background migration did not finish")
	}
	if err := lease.QueryRow("SELECT count(*) FROM chat_session WHERE chat_id = 'legacy-chat'").Scan(&imported); err != nil || imported != 1 {
		t.Fatalf("after releasing writer: %d imported, %v", imported, err)
	}
}
