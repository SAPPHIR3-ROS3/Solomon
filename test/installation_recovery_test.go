package test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	servercli "github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon/server"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestDaemonRecoversMissingCorruptAndStaleState(t *testing.T) {
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	port := strings.TrimPrefix(state.URL, "http://127.0.0.1:")
	t.Setenv("SOLOMON_SERVER_PORT", port)
	path, err := serverruntime.StatePath()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"missing", "corrupt", "stale PID"} {
		t.Run(scenario, func(t *testing.T) {
			switch scenario {
			case "missing":
				err = serverruntime.ClearState()
			case "corrupt":
				err = os.WriteFile(path, []byte("incomplete JSON"), 0600)
			case "stale PID":
				stale := state
				stale.PID++
				err = serverruntime.SaveState(stale)
			}
			if err != nil {
				t.Fatal(err)
			}
			recovered, err := serverruntime.LoadRunningState(context.Background())
			if err != nil || recovered.PID != state.PID || recovered.Home != state.Home {
				t.Fatalf("recovery=%+v err=%v", recovered, err)
			}
			saved, err := serverruntime.LoadState()
			if err != nil || saved.PID != state.PID {
				t.Fatalf("recovered state was not persisted: %+v %v", saved, err)
			}
		})
	}
}

func TestDaemonRecoveryRefusesAnotherHome(t *testing.T) {
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	originalHome := state.Home
	t.Setenv("SOLOMON_SERVER_PORT", strings.TrimPrefix(state.URL, "http://127.0.0.1:"))
	t.Setenv("SOLOMON_HOME", t.TempDir())
	if _, err := serverruntime.LoadRunningState(context.Background()); err == nil || !strings.Contains(err.Error(), "does not identify") {
		t.Fatalf("adopted another installation: %v", err)
	}
	if _, err := serverruntime.LoadState(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("foreign state was saved: %v", err)
	}
	// The fixture's shutdown must still operate in its original home.
	os.Setenv("SOLOMON_HOME", originalHome)
}

func TestDaemonLockPreventsSecondServerWithoutOverwritingState(t *testing.T) {
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	err := serverruntime.Run(context.Background(), serverruntime.Options{ListenAddr: "127.0.0.1:0"})
	if err == nil || !strings.Contains(err.Error(), "already owns") {
		t.Fatalf("second daemon result=%v", err)
	}
	saved, err := serverruntime.LoadState()
	if err != nil || saved.PID != state.PID || !saved.StartedAt.Equal(state.StartedAt) {
		t.Fatalf("second daemon changed state: %+v %v", saved, err)
	}
}

func TestServerCommandsReportErrorsAndStartIsIdempotent(t *testing.T) {
	if err := servercli.Run([]string{"unsupported"}); err == nil {
		t.Fatal("invalid subcommand reported success")
	}
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	if err := servercli.Run([]string{"start"}); err != nil {
		t.Fatalf("start running daemon: %v", err)
	}
	saved, _ := serverruntime.LoadState()
	if saved.PID != state.PID || !saved.StartedAt.Equal(state.StartedAt) {
		t.Fatal("idempotent start replaced daemon")
	}
}

func TestForegroundDaemonReportsOccupiedPort(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	occupied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "unrelated service") }))
	defer occupied.Close()
	t.Setenv("SOLOMON_SERVER_PORT", strings.TrimPrefix(occupied.URL, "http://127.0.0.1:"))
	if err := servercli.Run([]string{"run"}); err == nil || !strings.Contains(err.Error(), "listen tcp4") {
		t.Fatalf("occupied port reported success: %v", err)
	}
	if _, err := serverruntime.LoadState(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed bind left daemon state: %v", err)
	}
}

func TestInstallationRollbackPreservesIndependentCLIAndDesktopCopies(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "CLI with spaces")
	app := filepath.Join(dir, "Solomon.app")
	file := filepath.Join(app, "Contents", "MacOS", "desktop")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{cli: "old CLI", file: "old desktop"} {
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	backup, err := updater.BackupInstallation(cli, app)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	// In-place writes would also overwrite a backup made with hard links.
	for _, path := range []string{cli, file} {
		if err := os.WriteFile(path, []byte("broken replacement"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := backup.Restore(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{cli: "old CLI", file: "old desktop"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("rollback %s=%q, err=%v", path, got, err)
		}
	}
}

func TestFailedFirstInstallationRollsBackToAbsentFiles(t *testing.T) {
	target := filepath.Join(t.TempDir(), "new installation")
	backup, err := updater.BackupInstallation(target)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if err := os.WriteFile(target, []byte("failed new binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := backup.Restore(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed installation left a binary: %v", err)
	}
}

func TestCommitInstallRejectsBadStagingAndPreservesPreviousBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "solomon")
	oldBackup := target + ".bak"
	for _, path := range []string{target, oldBackup} {
		if err := os.WriteFile(path, []byte("valuable original"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, staged := range []string{filepath.Join(dir, "missing"), target} {
		if err := updater.CommitInstall(staged, target); err == nil {
			t.Fatalf("bad staging %s accepted", staged)
		}
	}
	for _, path := range []string{target, oldBackup} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "valuable original" {
			t.Fatalf("destroyed %s: %q %v", path, got, err)
		}
	}
}

func TestReleaseDownloadRequiresChecksumsAndCleansStaging(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOBIN", dir)
	t.Setenv("SOLOMON_BINARY", "")
	restore := updater.SetHTTPDownload(func(_ context.Context, url string) (*http.Response, error) {
		status := http.StatusOK
		if strings.HasSuffix(url, "/checksums.txt") {
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("download"))}, nil
	})
	defer restore()
	if _, _, err := updater.PrepareInstall(context.Background(), "v2099.1.0", io.Discard); err == nil || !strings.Contains(err.Error(), "required checksums") {
		t.Fatalf("unverified binary accepted: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed download leaked files: %v %v", entries, err)
	}
}

func TestDaemonRecoveryRejectsRedirects(t *testing.T) {
	requests := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; fmt.Fprint(w, `{ "ok": true }`) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer redirect.Close()
	if _, err := serverruntime.ReadHealth(context.Background(), serverruntime.State{URL: redirect.URL}); err == nil || requests != 0 {
		t.Fatalf("followed health redirect: requests=%d err=%v", requests, err)
	}
}
