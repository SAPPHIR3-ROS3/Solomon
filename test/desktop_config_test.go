package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopConfig_defersDevServerURLToRuntimeLauncher(t *testing.T) {
	path := filepath.Join("..", "gui", "desktop", "wails.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]string
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if got := config["frontend:dev:serverUrl"]; got != "" {
		t.Fatalf("desktop config hard-codes dev server URL %q", got)
	}
	if got, want := config["frontend:dir"], ".."; got != want {
		t.Fatalf("desktop frontend directory = %q, want %q", got, want)
	}
}
