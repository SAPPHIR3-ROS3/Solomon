package config

import (
	"os"
	"path/filepath"
	"testing"

	to "github.com/pelletier/go-toml/v2"
)

func TestModelVisibilityDefaultsEnabledAndCanToggle(t *testing.T) {
	r := &Root{}
	if !ModelEnabled(r, "OpenAI", "gpt-5") {
		t.Fatal("models should be enabled by default")
	}
	if err := SetModelEnabled(r, "OpenAI", "gpt-5", false); err != nil {
		t.Fatalf("disable model: %v", err)
	}
	if ModelEnabled(r, "OpenAI", "gpt-5") {
		t.Fatal("disabled model should not be enabled")
	}
	if got := HiddenModelIDs(r, "OpenAI", []string{"gpt-5", "gpt-4.1"}); len(got) != 1 || got[0] != "gpt-5" {
		t.Fatalf("hidden models = %#v, want [gpt-5]", got)
	}
	if err := SetModelEnabled(r, "OpenAI", "gpt-5", true); err != nil {
		t.Fatalf("enable model: %v", err)
	}
	if !ModelEnabled(r, "OpenAI", "gpt-5") || len(r.HiddenModels) != 0 {
		t.Fatal("enabled model should be removed from hidden preferences")
	}
}

func TestModelVisibilityRoundTripsThroughConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	r := &Root{Providers: map[string]*Provider{"OpenAI": {Name: "OpenAI"}}}
	if err := SetModelEnabled(r, "OpenAI", "gpt-5", false); err != nil {
		t.Fatalf("disable model: %v", err)
	}
	if err := Save(r); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "config.toml")); err != nil {
		t.Fatalf("config file was not written: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if ModelEnabled(loaded, "OpenAI", "gpt-5") {
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
	previousLister := RolesModelLister
	RolesModelLister = nil
	t.Cleanup(func() { RolesModelLister = previousLister })

	if err := UpdateModelVisibility("OpenAI", "gpt-5", false); err != nil {
		t.Fatalf("update model visibility: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	var file rootFile
	if err := to.Unmarshal(b, &file); err != nil {
		t.Fatalf("parse updated config: %v", err)
	}
	if got := file.HiddenModels["OpenAI"]; len(got) != 1 || got[0] != "gpt-5" {
		t.Fatalf("hidden models = %#v, want [gpt-5]", got)
	}
}
