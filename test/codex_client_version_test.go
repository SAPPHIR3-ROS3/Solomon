package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
)

func TestCodexClientVersionRefreshAndFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	calls := 0
	version := "0.999.0"
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "" {
			t.Error("registry request must not include credentials")
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	}))
	defer srv.Close()
	previous := codex.CodexVersionRegistryURL
	codex.CodexVersionRegistryURL = srv.URL
	defer func() { codex.CodexVersionRegistryURL = previous }()
	ctx := context.Background()
	check := func(force bool, want string) {
		t.Helper()
		if got := codex.ResolveClientVersion(ctx, force); got != want {
			t.Fatalf("version = %q, want %q", got, want)
		}
	}
	// A failed cold lookup uses the built-in version.
	status = http.StatusServiceUnavailable
	check(true, codex.ClientVersion)
	status = http.StatusOK
	check(true, "0.999.0")
	before := calls
	check(false, "0.999.0")
	if calls != before {
		t.Fatal("fresh cache queried registry")
	}
	version = "0.999.1"
	check(true, version)
	data, err := os.ReadFile(filepath.Join(home, "codex-client-version.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &saved); err != nil || saved.Version != version {
		t.Fatalf("persisted version: %s (%v)", data, err)
	}
	version = "1.0.0-alpha.1"
	check(true, "0.999.1")
	status = http.StatusServiceUnavailable
	check(true, "0.999.1")
	before = calls
	check(false, "0.999.1")
	if calls != before {
		t.Fatal("failed lookup retried immediately")
	}
}
