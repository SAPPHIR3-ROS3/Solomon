package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestModelVisibilityDefaultsEnabledAndCanToggle(t *testing.T) {
	r := &config.Root{}
	if !config.ModelEnabled(r, "OpenAI", "gpt-5") {
		t.Fatal("models should be enabled by default")
	}
	if err := config.SetModelEnabled(r, "OpenAI", "gpt-5", false); err != nil {
		t.Fatalf("disable model: %v", err)
	}
	if config.ModelEnabled(r, "OpenAI", "gpt-5") {
		t.Fatal("disabled model should not be enabled")
	}
	if got := config.HiddenModelIDs(r, "OpenAI", []string{"gpt-5", "gpt-4.1"}); len(got) != 1 || got[0] != "gpt-5" {
		t.Fatalf("hidden models = %#v, want [gpt-5]", got)
	}
	if err := config.SetModelEnabled(r, "OpenAI", "gpt-5", true); err != nil {
		t.Fatalf("enable model: %v", err)
	}
	if !config.ModelEnabled(r, "OpenAI", "gpt-5") || len(r.HiddenModels) != 0 {
		t.Fatal("enabled model should be removed from hidden preferences")
	}
}

func TestModelVisibilityRoundTripsThroughConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	r := &config.Root{Providers: map[string]*config.Provider{"OpenAI": {Name: "OpenAI"}}}
	if err := config.SetModelEnabled(r, "OpenAI", "gpt-5", false); err != nil {
		t.Fatalf("disable model: %v", err)
	}
	if err := config.Save(r); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "config.toml")); err != nil {
		t.Fatalf("config file was not written: %v", err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.ModelEnabled(loaded, "OpenAI", "gpt-5") {
		t.Fatal("hidden model preference did not survive config round trip")
	}
}

func TestUpdateModelVisibilitySkipsLiveRoleValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	configSource := `[providers.OpenAI]
base_url = 'https://api.openai.com/v1'
api_key = 'test'

[current]
provider = 'OpenAI'
model = 'gpt-5'

[roles.table]
characteristics = ['speed']

[[roles.subagent]]
provider = 'OpenAI'
model = 'gpt-5'
`
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configSource), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	previousLister := config.RolesModelLister
	config.RolesModelLister = nil
	t.Cleanup(func() { config.RolesModelLister = previousLister })

	if err := config.UpdateModelVisibility("OpenAI", "gpt-5", false); err != nil {
		t.Fatalf("update model visibility: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if string(b) != configSource {
		t.Fatal("visibility update rewrote the main config")
	}
	saved, err := config.ReadModelVisibility()
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.HiddenModels["OpenAI"]; len(got) != 1 || got[0] != "gpt-5" {
		t.Fatalf("hidden models = %#v, want [gpt-5]", got)
	}
}

func TestModelVisibilitySurvivesStaleConfigSave(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := &config.Root{Providers: map[string]*config.Provider{"OpenAI": {Name: "OpenAI"}}}
	// Existing preferences must be migrated without requiring hundreds of toggles.
	for i := 0; i < 400; i++ {
		_ = config.SetModelEnabled(root, "OpenAI", fmt.Sprintf("model-%d", i), false)
	}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	stale, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateModelVisibility("OpenAI", "new-model", false); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateModelVisibility("OpenAI", "model-0", true); err != nil {
		t.Fatal(err)
	}
	// Simulate a token refresh or model selection saving an older Root snapshot.
	if err := config.Save(stale); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.HiddenModels["OpenAI"]) != 400 {
		t.Fatalf("lost saved preferences: %d", len(loaded.HiddenModels["OpenAI"]))
	}
	if config.ModelEnabled(loaded, "OpenAI", "new-model") {
		t.Fatal("stale config save re-enabled model")
	}
	if !config.ModelEnabled(loaded, "OpenAI", "model-0") {
		t.Fatal("stale config save disabled re-enabled model")
	}
	// A model disappearing from one catalog must not delete its preference.
	_ = config.HiddenModelIDs(loaded, "OpenAI", []string{"model-1"})
	again, err := config.ReadModelVisibility()
	if err != nil {
		t.Fatal(err)
	}
	if config.ModelEnabled(again, "OpenAI", "model-399") {
		t.Fatal("catalog filtering deleted preference")
	}
}
