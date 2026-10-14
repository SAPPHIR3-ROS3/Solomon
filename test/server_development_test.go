package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestSavedDevServerSettingsControlDefaultStartup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	directory := filepath.Join(home, "gui")
	if err := os.MkdirAll(filepath.Join(directory, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "config.toml")
	if err := os.WriteFile(path, []byte("user_name='Fixture'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mode, _, err := config.ResolveServerDevelopment("normal", "")
	if err != nil || mode != "normal" {
		t.Fatalf("default: %s, %v", mode, err)
	}
	rawDirectory, _ := json.Marshal(directory)
	if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{
		"server.dev_mode": json.RawMessage("true"), "server.dev_directory": rawDirectory,
	}}); err != nil {
		t.Fatal(err)
	}
	mode, source, err := config.ResolveServerDevelopment("normal", "")
	if err != nil || mode != "dev" || source != directory {
		t.Fatalf("saved dev: %s %s, %v", mode, source, err)
	}
	if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{"server.dev_mode": json.RawMessage("false")}}); err != nil {
		t.Fatal(err)
	}
	mode, source, err = config.ResolveServerDevelopment("dev", directory)
	if err != nil || mode != "normal" || source != "" {
		t.Fatalf("restart preference: %s %s, %v", mode, source, err)
	}
	// A removed source folder must fail before a default startup/restart occurs.
	if err := os.WriteFile(path, []byte("[server]\ndev_mode=true\ndev_directory='/missing/gui'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := config.ResolveServerDevelopment("normal", ""); err == nil {
		t.Fatal("accepted missing dev directory")
	}
}
