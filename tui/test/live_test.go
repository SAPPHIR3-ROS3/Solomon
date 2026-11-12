package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/live"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/state"
)

func TestDefaultConfiguration(t *testing.T) {
	cfg := live.Default()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Action("composer", "ctrl+c") != "quit" {
		t.Fatal("emergency exit missing")
	}
	if cfg.Action("composer", "enter") != "send" {
		t.Fatal("send action missing")
	}
	if cfg.Action("suggestions", "tab") != "complete" {
		t.Fatal("context binding missing")
	}
}
func TestPartialKeyOverrideKeepsDefaults(t *testing.T) {
	cfg, err := live.Decode([]byte("{\"version\":1,\"keys\":{\"global\":{\"new_thread\":[\"ctrl+t\"]}}}"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Action("composer", "ctrl+t") != "new_thread" || cfg.Action("composer", "ctrl+n") != "" {
		t.Fatal("override was not applied")
	}
	if cfg.Action("composer", "enter") != "send" || cfg.Action("composer", "ctrl+q") != "quit" {
		t.Fatal("unrelated bindings lost")
	}
}
func TestInvalidConfiguration(t *testing.T) {
	cases := []string{
		"{\"version\":2}", "{\"unknown\":1}", "{} {}", "{\"colors\":{\"gold\":\"red\"}}", "{\"colors\":{\"password\":\"#000000\"}}",
		"{\"layout\":{\"home_width\":2}}", "{\"text\":{\"workspace\":\"bad\\u001bname\"}}",
		"{\"keys\":{\"global\":{\"new_thread\":[\"ctrl+q\"]}}}", "{\"keys\":{\"global\":{\"new_thread\":[\"ctrl+c\"]}}}",
		"{\"keys\":{\"composer\":{\"new_action\":[\"ctrl+y\"]}}}", "{\"keys\":{\"global\":{\"models\":[\"ctrl+m\"]}}}", "{\"keys\":null}",
	}
	for _, input := range cases {
		if _, err := live.Decode([]byte(input)); err == nil {
			t.Errorf("accepted invalid config: %s", input)
		}
	}
}
func TestWatcherRetainsLastValidAndRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interface.json")
	watcher := live.NewWatcher(path)
	if changed, err := watcher.Poll(); err != nil || !changed {
		t.Fatalf("initial defaults: %v %v", changed, err)
	}
	cfg := live.Default()
	cfg.Text["workspace"] = "Changed"
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if changed, err := watcher.Poll(); err != nil || !changed {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if changed, err := watcher.Poll(); err == nil || changed {
		t.Fatal("invalid config applied")
	}
	if watcher.Current.Text["workspace"] != "Changed" {
		t.Fatal("last valid config lost")
	}
	if changed, err := watcher.Poll(); err != nil || changed {
		t.Fatal("unchanged invalid file repeatedly reported")
	}
	cfg.Text["workspace"] = "Recovered"
	data, _ = json.Marshal(cfg)
	os.WriteFile(path, data, 0600)
	if changed, err := watcher.Poll(); err != nil || !changed {
		t.Fatal(err)
	}
	if watcher.Current.Text["workspace"] != "Recovered" {
		t.Fatal("recovery failed")
	}
}
func TestExportPreservesCustomFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "interface.json")
	if err := live.Export(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := live.Decode(data); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("custom"), 0600)
	if err := live.Export(path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "custom" {
		t.Fatal("custom configuration overwritten")
	}
}
func TestConfigurationPath(t *testing.T) {
	t.Setenv("SOLOMON_TUI_CONFIG", "")
	t.Setenv("SOLOMON_HOME", t.TempDir())
	path, err := live.Path()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "customization/tui/interface.json") {
		t.Fatal(path)
	}
	override := filepath.Join(t.TempDir(), "override.json")
	t.Setenv("SOLOMON_TUI_CONFIG", override)
	path, err = live.Path()
	if err != nil || path != override {
		t.Fatal(path, err)
	}
}
func TestStateRoundTripAndVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "state.json")
	original := map[string]any{"draft": "caffè 世界", "scroll": 42.0, "messages": []any{"user", "assistant"}}
	if err := state.Save(path, "handoff", original); err != nil {
		t.Fatal(err)
	}
	var restored map[string]any
	token, err := state.Load(path, &restored)
	if err != nil {
		t.Fatal(err)
	}
	if token != "handoff" || restored["draft"] != original["draft"] || restored["scroll"] != 42.0 {
		t.Fatal("state mismatch")
	}
	os.WriteFile(path, []byte("{\"version\":99,\"data\":{}}"), 0600)
	if _, err := state.Load(path, &restored); err == nil {
		t.Fatal("future state version accepted")
	}
}

func TestMouseEnabledByDefaultAndConfigurable(t *testing.T) {
	if !live.Default().Mouse {
		t.Fatal("mouse must be enabled by default")
	}
	cfg, err := live.Decode([]byte("{\"mouse\":false}"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mouse {
		t.Fatal("mouse=false was not applied")
	}
	cfg, err = live.Decode([]byte("{}"))
	if err != nil || !cfg.Mouse {
		t.Fatal("omitted mouse setting must inherit enabled default")
	}
}
