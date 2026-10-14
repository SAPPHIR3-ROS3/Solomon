package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGUIBackgroundPersistsAcrossOtherPreferences(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	path := filepath.Join(home, "config.toml")
	if err := os.WriteFile(path, []byte("user_name = 'Fixture'\n[gui.background]\nfuture = 'preserve'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := UpdateGUIBackgroundSettings(true); err != nil {
		t.Fatal(err)
	}
	yes := true
	if _, err := UpdateGUIChatSettings(GUIChatSettings{AutoCloseToolCalls: &yes}); err != nil {
		t.Fatal(err)
	}
	settings, err := ReadGUISettings()
	if err != nil || !settings.Background.KeepAwake {
		t.Fatalf("keep awake lost: %+v %v", settings.Background, err)
	}
	if err := UpdateGUIBackgroundSettings(false); err != nil {
		t.Fatal(err)
	}
	settings, err = ReadGUISettings()
	if err != nil || settings.Background.KeepAwake {
		t.Fatal("disable did not persist")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "preserve") {
		t.Fatal("unknown background field lost")
	}
}
