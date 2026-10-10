package test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func updateRequest(a *serverruntime.UpdateAPIForTest, method, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, "http://localhost/__solomon/update", strings.NewReader(body))
	a.ServeHTTP(w, r)
	return w
}

func waitUpdatePhase(t *testing.T, a *serverruntime.UpdateAPIForTest, phase string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		current, _ := a.Status()
		if current == phase {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("update never reached %s", phase)
}

func TestGUIUpdateRequiresDownloadAndExplicitRestart(t *testing.T) {
	a := serverruntime.NewUpdateAPIForTest(context.Background(), serverruntime.State{Version: "v2026.1008.0"})
	a.SetCheck(func(context.Context) updater.CheckResult {
		return updater.CheckResult{LatestTag: "v2026.1009.0", Newer: true}
	})
	downloadStarted, finishDownload := make(chan struct{}), make(chan struct{})
	a.SetPrepare(func(context.Context, string, io.Writer) (string, string, error) {
		close(downloadStarted)
		<-finishDownload
		return "staged", "target", nil
	})
	launches := 0
	a.SetLaunch(func(tag, staged, target string) error {
		launches++
		if tag != "v2026.1009.0" || staged != "staged" || target != "target" {
			t.Fatal("incorrect coordinator handoff")
		}
		return nil
	})
	updateRequest(a, http.MethodGet, "")
	waitUpdatePhase(t, a, "available")
	if launches != 0 {
		t.Fatal("checking installed an update")
	}
	if w := updateRequest(a, http.MethodPost, `{"action":"install"}`); w.Code != http.StatusConflict {
		t.Fatal(w.Code)
	}
	if w := updateRequest(a, http.MethodPost, `{"action":"download"}`); w.Code != http.StatusAccepted {
		t.Fatal(w.Code)
	}
	<-downloadStarted
	if w := updateRequest(a, http.MethodPost, `{"action":"download"}`); w.Code != http.StatusConflict {
		t.Fatal("duplicate download accepted")
	}
	close(finishDownload)
	waitUpdatePhase(t, a, "ready")
	if launches != 0 {
		t.Fatal("downloading restarted the application")
	}
	if w := updateRequest(a, http.MethodPost, `{"action":"install"}`); w.Code != http.StatusAccepted {
		t.Fatal(w.Code)
	}
	if w := updateRequest(a, http.MethodPost, `{"action":"install"}`); w.Code != http.StatusConflict {
		t.Fatal("duplicate restart accepted")
	}
	if launches != 1 {
		t.Fatalf("launches: %d", launches)
	}
}

func TestGUIUpdateFailuresCanBeRetried(t *testing.T) {
	a := serverruntime.NewUpdateAPIForTest(context.Background(), serverruntime.State{Version: "dev"})
	a.SetStatus("available", "v2026.1009.0", "")
	a.SetPrepare(func(context.Context, string, io.Writer) (string, string, error) {
		return "", "", errors.New("checksum mismatch")
	})
	updateRequest(a, http.MethodPost, `{"action":"download"}`)
	waitUpdatePhase(t, a, "available")
	if _, message := a.Status(); message != "checksum mismatch" {
		t.Fatal(message)
	}
	a.SetStatus("ready", "v2026.1009.0", "")
	a.SetLaunch(func(string, string, string) error { return errors.New("cannot launch helper") })
	if w := updateRequest(a, http.MethodPost, `{"action":"install"}`); w.Code != http.StatusInternalServerError {
		t.Fatal(w.Code)
	}
	if phase, message := a.Status(); phase != "ready" || message != "cannot launch helper" {
		t.Fatal(phase, message)
	}
}

func TestGUIUpdateReportsCoordinatorFailure(t *testing.T) {
	a := serverruntime.NewUpdateAPIForTest(context.Background(), serverruntime.State{})
	staged := filepath.Join(t.TempDir(), ".solomon-update-test")
	a.SetStatus("restarting", "", staged)
	if err := os.WriteFile(staged+".error", []byte("runtime could not restart"), 0600); err != nil {
		t.Fatal(err)
	}
	w := updateRequest(a, http.MethodGet, "")
	phase, _ := a.Status()
	if !strings.Contains(w.Body.String(), "runtime could not restart") || phase != "available" {
		t.Fatal(w.Body.String())
	}
}

func TestGUIUpdateRejectsForeignOriginAndInvalidActions(t *testing.T) {
	a := serverruntime.NewUpdateAPIForTest(context.Background(), serverruntime.State{})
	r := httptest.NewRequest(http.MethodPost, "http://localhost/__solomon/update", strings.NewReader(`{"action":"download"}`))
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatal(w.Code)
	}
	if w := updateRequest(a, http.MethodPost, `{"action":"unexpected"}`); w.Code != http.StatusBadRequest {
		t.Fatal(w.Code)
	}
	if w := updateRequest(a, http.MethodDelete, ""); w.Code != http.StatusMethodNotAllowed {
		t.Fatal(w.Code)
	}
}

func TestGUIUpdateCachesCheckWhenAlreadyCurrent(t *testing.T) {
	a := serverruntime.NewUpdateAPIForTest(context.Background(), serverruntime.State{Version: "v2026.1009.0"})
	checks := 0
	a.SetCheck(func(context.Context) updater.CheckResult {
		checks++
		return updater.CheckResult{LatestTag: "v2026.1009.0"}
	})
	updateRequest(a, http.MethodGet, "")
	waitUpdatePhase(t, a, "idle")
	for i := 0; i < 5; i++ {
		updateRequest(a, http.MethodGet, "")
	}
	phase, _ := a.Status()
	if checks != 1 || phase != "idle" {
		t.Fatalf("checks=%d phase=%s", checks, phase)
	}
}
