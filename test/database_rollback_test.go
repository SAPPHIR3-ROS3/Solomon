package test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
)

func TestDatabaseUpdaterRollsBackAfterFirstWrite(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	setSolomonHome(t, home)
	path := filepath.Join(home, "AGENTS.md")
	target := filepath.Join(home, "solomon")
	staged := filepath.Join(home, "staged")
	for file, value := range map[string]string{path: "before", target: "previous", staged: "next"} {
		if err := os.WriteFile(file, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	db := openTestDatabase(t, home, database.Data)
	if _, err := database.CaptureText(ctx, db, path); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("new binary failed health check after writing")
	restored := false
	err := lifecycle.ApplyUpdateForTest(ctx, "next", lifecycle.UpdateOpsForTest{
		PreserveStorage: true,
		Prepare:         func() (string, string, error) { return staged, target, nil },
		Stop:            func(string) error { return nil },
		Quiesce:         func() error { return nil },
		Commit:          func(_, target string) error { return os.WriteFile(target, []byte("next"), 0o600) },
		Start: func(string) error {
			if err := os.WriteFile(path, []byte("after"), 0o600); err != nil {
				return err
			}
			db, err := database.Open(ctx, home, database.Data)
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := database.CaptureText(ctx, db, path); err != nil {
				return err
			}
			logs, err := database.Open(ctx, home, database.Logs)
			if err != nil {
				return err
			}
			_ = logs.Close()
			return failure
		},
		Restore: func(string) error { restored = true; return nil },
	})
	if !errors.Is(err, failure) || !restored {
		t.Fatalf("rollback got %v, restarted old binary: %t", err, restored)
	}
	for file, value := range map[string]string{path: "before", target: "previous"} {
		if actual, err := os.ReadFile(file); err != nil || string(actual) != value {
			t.Fatalf("rollback got %q, %v", actual, err)
		}
	}
	db = openTestDatabase(t, home, database.Data)
	var content []byte
	if err := db.QueryRow("SELECT content FROM managed_text WHERE path = ?", path).Scan(&content); err != nil || string(content) != "before" {
		t.Fatalf("database rollback got %q, %v", content, err)
	}
	logPath, _ := database.Path(home, database.Logs)
	if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback did not remove new log database: %v", err)
	}
}

func TestDatabaseMaintenanceWaitsForActiveConnections(t *testing.T) {
	home := t.TempDir()
	db := openTestDatabase(t, home, database.Data)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	release, err := database.AcquireMaintenance(ctx, home)
	if err == nil {
		_ = release()
		t.Fatal("maintenance entered while a database connection was active")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	release, err = database.AcquireMaintenance(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancelOpen := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancelOpen()
	if reopened, err := database.Open(ctx, home, database.Data); err == nil {
		_ = reopened.Close()
		t.Fatal("writer opened during maintenance")
	}
}

func TestDatabaseRollsBackUncommittedProcessExit(t *testing.T) {
	home := t.TempDir()
	db := openTestDatabase(t, home, database.Data)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(binary, "-test.run=^TestDatabaseExitFixture$")
	child.Env = append(os.Environ(), "SOLOMON_DATABASE_CRASH_FIXTURE="+home)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("crash fixture: %v, %s", err, output)
	}
	db = openTestDatabase(t, home, database.Data)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM managed_text").Scan(&count); err != nil || count != 0 {
		t.Fatalf("uncommitted transaction became visible: %d, %v", count, err)
	}
}

func TestDatabaseExitFixture(t *testing.T) {
	home := os.Getenv("SOLOMON_DATABASE_CRASH_FIXTURE")
	if home == "" {
		return
	}
	db, err := database.Open(context.Background(), home, database.Data)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO managed_text(path, content, content_sha256) VALUES (?, zeroblob(4194304), ?)", "/fixture", strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestDatabasePoolReusesConnectionAndYieldsToMaintenance(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	first, err := database.BorrowData(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec("CREATE TEMP TABLE connection_probe(value INTEGER)"); err != nil {
		_ = first.Close()
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := database.BorrowData(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	err = second.QueryRow("SELECT count(*) FROM connection_probe").Scan(&count)
	_ = second.Close()
	if err != nil || count != 0 {
		t.Fatalf("connection was not reused: %v", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	release, err := database.AcquireMaintenance(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}
