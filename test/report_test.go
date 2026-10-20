package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestReportRedactsAndKeepsNewestLogs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	redacted := logging.RedactReport("user " + home + " mail a@b.co token Bearer super-secret sk-abcdefghijklmnopqrstuvwxyz from 10.1.2.3")
	if strings.Contains(redacted, home) || strings.Contains(redacted, "a@b.co") || strings.Contains(redacted, "super-secret") || strings.Contains(redacted, "sk-abcdefghijklmnopqrstuvwxyz") || strings.Contains(redacted, "10.1.2.3") {
		t.Fatalf("redaction missed data: %s", redacted)
	}
	dir := t.TempDir()
	logging.LogInit(logging.INFO_LOG_LEVEL)
	restore := logging.IsolateNotificationsForTest(dir)
	defer restore()
	if err := logging.Configure(logging.Config{Dir: dir, WriteFile: true, Retention: 7}); err != nil {
		t.Fatal(err)
	}
	var block strings.Builder
	for i := 0; i < 40; i++ {
		block.WriteString("[2026-10-20T00:00:00Z][DEBUG][a.go:1][main] skip\n")
		block.WriteString("[2026-10-20T00:00:00Z][INFO][a.go:1][main] " + strings.Repeat("i", 1000) + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-10-20.log"), []byte(block.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	logs, err := logging.ReportLogs()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs, "DEBUG") || len(logs) == 0 || len(logs) > logging.ReportByteLimit {
		t.Fatalf("report logs = %d bytes, debug=%v", len(logs), strings.Contains(logs, "DEBUG"))
	}
}

func TestReportAPIPostsAnonymizedBody(t *testing.T) {
	var got map[string]string
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer remote.Close()
	restore := server.SetReportDestinationForTest(remote.URL)
	defer restore()
	dir := t.TempDir()
	logging.LogInit(logging.INFO_LOG_LEVEL)
	undo := logging.IsolateNotificationsForTest(dir)
	defer undo()
	if err := logging.Configure(logging.Config{Dir: dir, WriteFile: true}); err != nil {
		t.Fatal(err)
	}
	line := "[2026-10-20T00:00:00Z][ERROR][a.go:1][main] failed at /tmp/secret-path token sk-abcdefghijklmnopqrstuvwxyz\n"
	if err := os.WriteFile(filepath.Join(dir, "2026-10-20.log"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/__solomon/report", strings.NewReader(`{"note":"see a@b.co"}`))
	response := httptest.NewRecorder()
	server.ReportHandlerForTest()(response, request)
	if response.Code != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status %d %s", response.Code, body)
	}
	if got["note"] != "see <email>" || strings.Contains(got["logs"], "sk-abcdefghijklmnopqrstuvwxyz") || strings.Contains(got["logs"], "/tmp/secret-path") {
		t.Fatalf("payload %#v", got)
	}
}
