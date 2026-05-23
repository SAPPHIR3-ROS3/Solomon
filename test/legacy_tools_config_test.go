package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func writeTestConfig(t *testing.T, home, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigToolsSectionLoadMerge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	writeTestConfig(t, home, `
legacy_tools = true
legacy_tools_force = true

[tools]
legacy = false

[providers.p]
base_url = "http://127.0.0.1:9"
api_key = "k"

[current]
provider = "p"
model = "m"
`)
	r, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !r.Tools.Legacy {
		t.Fatal("expected merged legacy=true from root")
	}
	if !r.Tools.LegacyForce {
		t.Fatal("expected merged legacy_force=true from root")
	}
}

func TestConfigToolsSectionSaveShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	writeTestConfig(t, home, `
[providers.p]
base_url = "http://127.0.0.1:9"
api_key = "k"

[current]
provider = "p"
model = "m"
`)
	r, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	r.Tools = config.Tools{Legacy: true, LegacyForce: false}
	if err := config.Save(r); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if strings.Contains(text, "\nlegacy_tools ") || strings.HasPrefix(strings.TrimSpace(text), "legacy_tools ") {
		t.Fatalf("deprecated root legacy_tools written: %q", text)
	}
	if !strings.Contains(text, "[tools]") || !strings.Contains(text, "legacy = true") {
		t.Fatalf("want [tools] legacy = true in %q", text)
	}
}
