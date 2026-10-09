package test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	toml "github.com/pelletier/go-toml/v2"
)

func guiConfigDocument(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestGUISettingsPersistSectionsAndPreserveBackendConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	path := filepath.Join(home, "config.toml")
	// An invalid runtime role configuration must not block presentation settings.
	source := "user_name = 'Fixture'\n[providers.OpenAI]\napi_key = 'fixture-secret'\n[roles.table]\ncharacteristics = ['speed']\n[[roles.subagent]]\nprovider = 'OpenAI'\nmodel = 'unknown'\n[unknown]\nkeep = ['a', 'b']\n[gui.chat]\nfuture_preference = 'preserve me'\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	before := guiConfigDocument(t, path)
	yes, no := true, false
	if _, err := config.UpdateGUIChatSettings(config.GUIChatSettings{AutoCloseToolCalls: &yes}); err != nil {
		t.Fatal(err)
	}
	gui, err := config.UpdateGUIChatSettings(config.GUIChatSettings{StartToolCallsCollapsed: &no})
	if err != nil {
		t.Fatal(err)
	}
	if gui.Chat.AutoCloseToolCalls == nil || !*gui.Chat.AutoCloseToolCalls || gui.Chat.StartToolCallsCollapsed == nil || *gui.Chat.StartToolCallsCollapsed {
		t.Fatalf("unexpected preferences: %+v", gui.Chat)
	}
	after := guiConfigDocument(t, path)
	sections := after["gui"].(map[string]any)
	for _, name := range []string{"chat", "models", "docs"} {
		if _, exists := sections[name]; !exists {
			t.Fatalf("missing gui.%s", name)
		}
	}
	chat := sections["chat"].(map[string]any)
	if chat["auto_close_tool_calls"] != true || chat["start_tool_calls_collapsed"] != false || chat["future_preference"] != "preserve me" {
		t.Fatalf("chat settings: %#v", chat)
	}
	delete(before, "gui")
	delete(after, "gui")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("GUI update changed backend config")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions: %v", info.Mode())
	}
}

func TestGUISettingsMigrateModelPreferencesIntoConfig(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "named-provider", true: "legacy-provider"}[legacy], func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("SOLOMON_HOME", home)
			source := "[providers.OpenAI]\napi_key = 'fixture'\n[hidden_models]\nOpenAI = ['old-root']\n"
			if legacy {
				source = "[[providers]]\nname = 'OpenAI'\napi_key = 'fixture'\n[hidden_models]\nOpenAI = ['old-root']\n"
			}
			path := filepath.Join(home, "config.toml")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "model-visibility.json"), []byte(`{"OpenAI":["old-sidecar"]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			no := false
			if _, err := config.UpdateGUIChatSettings(config.GUIChatSettings{AutoCloseToolCalls: &no}); err != nil {
				t.Fatal(err)
			}
			document := guiConfigDocument(t, path)
			if _, exists := document["hidden_models"]; exists {
				t.Fatal("legacy root preference was not migrated")
			}
			if err := config.UpdateModelVisibility("OpenAI", "old-sidecar", true); err != nil {
				t.Fatal(err)
			}
			gui, err := config.ReadGUISettings()
			if err != nil {
				t.Fatal(err)
			}
			if len(gui.Models.HiddenModels) != 0 {
				t.Fatalf("sidecar overrode empty GUI config: %#v", gui.Models.HiddenModels)
			}
			if err := config.UpdateModelVisibility("OpenAI", "new-model", false); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "[gui.models.hidden_models]") {
				t.Fatal("model preferences were not saved under gui.models")
			}
		})
	}
}

func TestGUISettingsConcurrentUpdatesAndStaleSaves(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	root.Providers = map[string]*config.Provider{"OpenAI": {Name: "OpenAI"}}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	stale, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	var wg sync.WaitGroup
	errors := make(chan error, 3)
	wg.Add(3)
	go func() {
		defer wg.Done()
		_, err := config.UpdateGUIChatSettings(config.GUIChatSettings{AutoCloseToolCalls: &yes})
		errors <- err
	}()
	go func() {
		defer wg.Done()
		_, err := config.UpdateGUIChatSettings(config.GUIChatSettings{StartToolCallsCollapsed: &yes})
		errors <- err
	}()
	go func() { defer wg.Done(); errors <- config.UpdateModelVisibility("OpenAI", "hidden", false) }()
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Repeated stale runtime saves must not overwrite newer GUI preferences.
	for i := 0; i < 2; i++ {
		if err := config.Save(stale); err != nil {
			t.Fatal(err)
		}
	}
	gui, err := config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	if gui.Chat.AutoCloseToolCalls == nil || !*gui.Chat.AutoCloseToolCalls || gui.Chat.StartToolCallsCollapsed == nil || !*gui.Chat.StartToolCallsCollapsed {
		t.Fatal("stale config save overwrote chat preferences")
	}
	if got := gui.Models.HiddenModels["OpenAI"]; len(got) != 1 || got[0] != "hidden" {
		t.Fatalf("lost models preferences: %#v", got)
	}
}
