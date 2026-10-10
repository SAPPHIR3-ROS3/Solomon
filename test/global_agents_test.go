package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestGlobalAgentsCreateEditAndClear(t *testing.T) {
	home := filepath.Join(t.TempDir(), "solomon")
	t.Setenv("SOLOMON_HOME", home)
	a := serverruntime.GlobalAgentsHandlerForTest()
	for _, step := range []struct {
		method, body, content string
	}{
		{http.MethodGet, "", ""},
		{http.MethodPost, `{"content":"  Use Italian.\n"}`, "  Use Italian.\n"},
		{http.MethodGet, "", "  Use Italian.\n"},
		{http.MethodPost, `{"content":""}`, ""},
		{http.MethodGet, "", ""},
	} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest(step.method, "/__solomon/globalAgents", strings.NewReader(step.body)))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", step.method, w.Code, w.Body.String())
		}
		var payload struct {
			GlobalAgents struct {
				Path, Content string
			}
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(home, "AGENTS.md")
		if payload.GlobalAgents.Path != path || payload.GlobalAgents.Content != step.content {
			t.Fatalf("unexpected response: %+v", payload)
		}
		content, err := os.ReadFile(path)
		if err != nil || string(content) != step.content {
			t.Fatalf("file content %q, error %v", content, err)
		}
	}
}

func TestGlobalAgentsRejectInvalidUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	path := filepath.Join(home, "AGENTS.md")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := serverruntime.GlobalAgentsHandlerForTest()
	for _, body := range []string{`{}`, `{"content":null}`, `{"content":42}`, `{`} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/__solomon/globalAgents", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %q: status %d", body, w.Code)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "keep" {
		t.Fatalf("existing content changed: %q, %v", content, err)
	}
}
