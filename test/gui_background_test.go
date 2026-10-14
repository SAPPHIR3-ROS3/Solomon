package test

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
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
	if err := config.UpdateGUIBackgroundSettings(true); err != nil {
		t.Fatal(err)
	}
	yes := true
	if _, err := config.UpdateGUIChatSettings(config.GUIChatSettings{AutoCloseToolCalls: &yes}); err != nil {
		t.Fatal(err)
	}
	settings, err := config.ReadGUISettings()
	if err != nil || !settings.Background.KeepAwake {
		t.Fatalf("keep awake lost: %+v %v", settings.Background, err)
	}
	if err := config.UpdateGUIBackgroundSettings(false); err != nil {
		t.Fatal(err)
	}
	settings, err = config.ReadGUISettings()
	if err != nil || settings.Background.KeepAwake {
		t.Fatal("disable did not persist")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "preserve") {
		t.Fatal("unknown background field lost")
	}
}
