package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
)

func TestDatabaseKindsAndBackup(t *testing.T) {
	ctx := context.Background()
	home := filepath.Join(t.TempDir(), "home #1")
	data := openTestDatabase(t, home, database.Data)
	logs := openTestDatabase(t, home, database.Logs)
	if _, err := data.Exec("SELECT * FROM log_event"); err == nil {
		t.Fatal("data database contains log tables")
	}
	if _, err := logs.Exec("SELECT * FROM managed_text"); err == nil {
		t.Fatal("log database contains data tables")
	}
	event := database.LogEvent{Time: time.Unix(1700000000, 730), Source: "daemon", Level: "INFO", Message: "ready"}
	id, err := database.AppendLog(ctx, logs, event)
	if err != nil {
		t.Fatal(err)
	}
	if retry, err := database.AppendLog(ctx, logs, event); err != nil || retry != id {
		t.Fatalf("retry got %s, %v", retry, err)
	}
	backupHome := t.TempDir()
	backupPath, err := database.Path(backupHome, database.Logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Backup(ctx, logs, backupPath); err != nil {
		t.Fatal(err)
	}
	if err := database.Backup(ctx, logs, backupPath); !errors.Is(err, os.ErrExist) {
		t.Fatalf("backup overwrite got %v", err)
	}
	if _, err := logs.Exec("DELETE FROM log_event"); err != nil {
		t.Fatal(err)
	}
	backedUp := openTestDatabase(t, backupHome, database.Logs)
	var count, epoch int64
	if err := backedUp.QueryRow("SELECT count(*), max(epoch_ns) FROM log_event WHERE sha256 = ?", id).Scan(&count, &epoch); err != nil {
		t.Fatal(err)
	}
	if count != 1 || epoch != event.Time.UnixNano() {
		t.Fatalf("backup contains %d rows at epoch %d", count, epoch)
	}
}

func TestDatabaseManagedText(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	db := openTestDatabase(t, home, database.Data)
	path := filepath.Join(home, "AGENTS.md")
	for _, body := range []string{"", "first", "current\r\n"} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := database.CaptureText(ctx, db, path); err != nil || string(got) != body {
			t.Fatalf("capture got %q, %v", got, err)
		}
	}
	if err := database.RestoreText(ctx, db, path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("restore overwrite got %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CaptureText(ctx, db, path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("capture missing file got %v", err)
	}
	if err := database.RestoreText(ctx, db, path); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "current\r\n" {
		t.Fatalf("restore got %q, %v", got, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE managed_text SET content = ?", []byte("changed")); err != nil {
		t.Fatal(err)
	}
	if err := database.RestoreText(ctx, db, path); err == nil {
		t.Fatal("restored a copy with a mismatched checksum")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed restore left a file or returned %v", err)
	}
}

func TestDatabaseConcurrentInitialization(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	const writers = 8
	errs := make(chan error, writers)
	var group sync.WaitGroup
	for i := range writers {
		group.Go(func() {
			db, err := database.Open(ctx, home, database.Logs)
			if err != nil {
				errs <- err
				return
			}
			defer db.Close()
			_, err = database.AppendLog(ctx, db, database.LogEvent{Time: time.Unix(1700000000, int64(i)), Source: "test"})
			errs <- err
		})
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	db := openTestDatabase(t, home, database.Logs)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM log_event").Scan(&count); err != nil || count != writers {
		t.Fatalf("concurrent writes got %d, %v", count, err)
	}
}

func TestDatabaseRejectsUnknownVersion(t *testing.T) {
	home := t.TempDir()
	db := openTestDatabase(t, home, database.Data)
	if _, err := db.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	if newer, err := database.Open(context.Background(), home, database.Data); err == nil {
		newer.Close()
		t.Fatal("accepted unsupported schema")
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 999 {
		t.Fatalf("unsupported schema version changed to %d, %v", version, err)
	}
}

func openTestDatabase(t *testing.T, home string, kind database.Kind) *sql.DB {
	t.Helper()
	db, err := database.Open(context.Background(), home, kind)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDatabaseGlobalInstructionsRequireExplicitRestore(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	path, err := paths.EnsureGlobalAgentsPath()
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("original")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := paths.ReadGlobalAgents(ctx, path); err != nil || string(got) != string(body) {
		t.Fatalf("read got %q, %v", got, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := paths.EnsureGlobalAgentsPath(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("startup recreated the file or failed: %v", err)
	}
	if err := paths.RestoreGlobalAgents(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(body) {
		t.Fatalf("read restored file got %q, %v", got, err)
	}
}

func TestDatabaseRegistryVerificationIsReadOnly(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	legacy, err := paths.ProjectsMapPath()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	id := project.IDHexFromRoot(root)
	before, err := json.Marshal(map[string]string{root: id})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, before, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := project.LoadMap(legacy)
	if err != nil || m[root] != id {
		t.Fatalf("import got %v, %v", m, err)
	}
	if err := project.UnregisterProject(legacy, id, false); err != nil {
		t.Fatal(err)
	}
	difference, err := project.VerifyLegacyRegistry(ctx, legacy)
	if err != nil || len(difference) != 1 || difference[0].RootPath != root || difference[0].StoredID != nil {
		t.Fatalf("verification got %v, %v", difference, err)
	}
	stored, err := project.LoadMap(legacy)
	if err != nil || len(stored) != 0 {
		t.Fatalf("verification recreated legacy project: %v, %v", stored, err)
	}
	got, err := os.ReadFile(legacy)
	if err != nil || string(got) != string(before) {
		t.Fatalf("verification modified the legacy map: %q, %v", got, err)
	}
}

func TestDatabaseLoggingReadLastIncludesLegacyAndCurrentEvents(t *testing.T) {
	home := t.TempDir()
	directory := filepath.Join(home, "logs")
	logging.LogInit(logging.INFO_LOG_LEVEL)
	if err := logging.Configure(logging.Config{DatabaseHome: home, Dir: directory, WriteFile: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logging.Configure(logging.Config{WriteConsole: true}) })
	legacy := filepath.Join(directory, time.Now().Format("2006-01-02")+".log")
	if err := os.WriteFile(legacy, []byte("legacy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logging.Log(logging.INFO_LOG_LEVEL, "new\nsecond line")
	logging.Log(logging.INFO_LOG_LEVEL, "latest")
	lines, err := logging.ReadLast(4)
	if err != nil || len(lines) != 4 || lines[0] != "legacy" || lines[2] != "second line" || !strings.HasSuffix(lines[3], "latest") {
		t.Fatalf("log tail got %q, %v", lines, err)
	}
	raw, err := os.ReadFile(legacy)
	if err != nil || string(raw) != "legacy\n" {
		t.Fatalf("new logs were appended to the legacy file: %q, %v", raw, err)
	}
}

func TestDatabaseProcessOutputKeepsLinesAcrossWrites(t *testing.T) {
	home := t.TempDir()
	writer, err := logging.OpenProcessLog(home, "test-sidecar")
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range []string{"spl", "it\nsecond\npartial"} {
		if n, err := writer.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("write got %d, %v", n, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	db := openTestDatabase(t, home, database.Logs)
	rows, err := db.Query("SELECT message FROM log_event WHERE source = ? ORDER BY epoch_ns", "test-sidecar")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var messages []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(messages, ",") != "split,second,partial" {
		t.Fatalf("unexpected output: %q", messages)
	}
}
