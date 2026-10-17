package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
)

func TestClaudeCodeVersion_FallbackWithoutCache(t *testing.T) {
	claude.ResetVersionCacheForTest()
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if got := claude.Version(); got != claude.FallbackVersion {
		t.Fatalf("version: got %q want fallback %q", got, claude.FallbackVersion)
	}
}

func TestClaudeCodeVersion_UsesFreshCache(t *testing.T) {
	claude.ResetVersionCacheForTest()
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	cachePath := filepath.Join(home, "cache", "claude-code-version.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"version":    "9.9.9",
		"fetched_at": time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := claude.Version(); got != "9.9.9" {
		t.Fatalf("version: got %q want 9.9.9", got)
	}
}

func TestClaudeCodeVersion_StaleCacheFallsBackWithoutNetwork(t *testing.T) {
	claude.ResetVersionCacheForTest()
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	cachePath := filepath.Join(home, "cache", "claude-code-version.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"version":    "8.8.8",
		"fetched_at": time.Now().Add(-48 * time.Hour).UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got := claude.Version()
	if got != "8.8.8" && got != claude.FallbackVersion {
		t.Fatalf("version: got %q want stale cache or fallback", got)
	}
}
