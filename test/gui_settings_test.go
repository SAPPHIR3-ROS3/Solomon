package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	// Windows reports read/write bits rather than Unix permission modes.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
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

func TestGUIResponseLanguageDefaultsAndUsesRuntimeConfig(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	root.Providers = map[string]*config.Provider{"OpenAI": {Name: "OpenAI"}}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	gui, err := config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	if gui.General.ResponseLanguage != config.DefaultResponseLanguage {
		t.Fatalf("unexpected default: %q", gui.General.ResponseLanguage)
	}
	_, err = config.UpdateGUIPreferences(config.GUISettingsPatch{General: &config.GUIGeneralPatch{ResponseLanguage: stringPointer("Italian")}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EffectiveResponseLanguage() != "Italian" {
		t.Fatal("runtime did not use GUI language")
	}
	yes := true
	if _, err := config.UpdateGUIChatSettings(config.GUIChatSettings{AutoCloseToolCalls: &yes}); err != nil {
		t.Fatal(err)
	}
	loaded, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EffectiveResponseLanguage() != "Italian" {
		t.Fatal("chat update changed runtime language")
	}
	for _, language := range []string{"", " ", "Italian\nEnglish", strings.Repeat("x", 257)} {
		if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{General: &config.GUIGeneralPatch{ResponseLanguage: &language}}); err == nil {
			t.Fatalf("accepted invalid language: %q", language)
		}
	}
}

func stringPointer(value string) *string { return &value }

func TestGUIAdvancedAndNameTargetedPersistence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	path := filepath.Join(home, "config.toml")
	source := "user_name='Old'\nresponse_language='Italian'\nautoupdate=false\n[providers.OpenAI]\napi_key='secret'\n[tool_output]\nmax_bytes=65536\nmax_lines=2048\nfuture_key='keep'\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	advanced := map[string]json.RawMessage{
		"compaction_threshold_tokens": json.RawMessage("65536"),
		"subagent_timeout_minutes":    json.RawMessage("35"),
		"log_level":                   json.RawMessage(`"debug"`),
		"research_max_rounds":         json.RawMessage("10"),
		"research_max_urls_per_round": json.RawMessage("5"),
		"research_max_content_chars":  json.RawMessage("20000"),
		"tool_output.max_bytes":       json.RawMessage("8192"),
		"tool_output.max_lines":       json.RawMessage("512"),
	}
	gui, err := config.UpdateGUIPreferences(config.GUISettingsPatch{General: &config.GUIGeneralPatch{UserName: stringPointer(" New ")}, Advanced: advanced})
	if err != nil {
		t.Fatal(err)
	}
	if gui.General.UserName != "New" || gui.General.ResponseLanguage != "Italian" {
		t.Fatalf("general: %+v", gui.General)
	}
	reread, err := config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	// JSON compares numeric types after reread (int and int64 serialize identically).
	encoded, _ := json.Marshal(gui)
	reencoded, _ := json.Marshal(reread)
	if string(encoded) != string(reencoded) {
		t.Fatal("advanced preferences did not round trip")
	}
	document := guiConfigDocument(t, path)
	if document["autoupdate"] != false {
		t.Fatal("automatic updates changed")
	}
	output := document["tool_output"].(map[string]any)
	if output["max_bytes"] != int64(8192) || output["max_lines"] != int64(512) || output["future_key"] != "keep" {
		t.Fatalf("tool output: %#v", output)
	}
	if document["providers"].(map[string]any)["OpenAI"].(map[string]any)["api_key"] != "secret" {
		t.Fatal("credentials changed")
	}
	for _, invalid := range []map[string]json.RawMessage{
		{"log_level": json.RawMessage(`"invalid"`)},
		{"subagent_timeout_minutes": json.RawMessage("0")},
		{"subagent_timeout_minutes": json.RawMessage("1.5")},
		{"research_max_rounds": json.RawMessage(`"8"`)},
		{"tool_output.max_bytes": json.RawMessage("null")},
		{"compaction_threshold_tokens": json.RawMessage("1")},
		{"autoupdate": json.RawMessage("true")},
	} {
		if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{General: &config.GUIGeneralPatch{UserName: stringPointer("Must not save")}, Advanced: invalid}); err == nil {
			t.Fatalf("accepted invalid patch: %v", invalid)
		}
		if !reflect.DeepEqual(document, guiConfigDocument(t, path)) {
			t.Fatal("invalid patch partially saved")
		}
	}
}

func TestGUIDevServerPreferencesPersistAndPreserveConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	directory := filepath.Join(home, "gui")
	if err := os.MkdirAll(filepath.Join(directory, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	root := config.EmptyRoot()
	root.Providers = map[string]*config.Provider{"OpenAI": {Name: "OpenAI", APIKey: "secret"}}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "config.toml")
	before := guiConfigDocument(t, path)
	if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{"server.dev_mode": json.RawMessage("true")}}); err == nil {
		t.Fatal("enabled dev without GUI directory")
	}
	if !reflect.DeepEqual(before, guiConfigDocument(t, path)) {
		t.Fatal("failed activation changed config")
	}
	rawDirectory, _ := json.Marshal(directory)
	gui, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{
		"server_port": json.RawMessage("64500"), "server.dev_directory": rawDirectory, "server.dev_mode": json.RawMessage("true"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if gui.Advanced["server.dev_mode"] != true || gui.Advanced["server.dev_directory"] != directory {
		t.Fatalf("GUI values: %#v", gui.Advanced)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ServerPort != 64500 || loaded.Server.DevMode == nil || !*loaded.Server.DevMode || loaded.Server.DevDirectory != directory {
		t.Fatalf("runtime config did not use server settings: %+v", loaded.Server)
	}
	// A normal config save must retain the new server fields.
	if err := config.Save(loaded); err != nil {
		t.Fatal(err)
	}
	mode, source, err := config.ResolveServerDevelopment("normal", "")
	if err != nil || mode != "dev" || source != directory {
		t.Fatalf("startup mode: %s, %s, %v", mode, source, err)
	}
	document := guiConfigDocument(t, path)
	for _, patch := range []map[string]json.RawMessage{
		{"server_port": json.RawMessage("0")}, {"server_port": json.RawMessage("65536")},
		{"server.dev_mode": json.RawMessage("null")}, {"server.dev_mode": json.RawMessage(`"true"`)},
		{"server.dev_directory": json.RawMessage(`""`)}, {"server.dev_directory": json.RawMessage(`"relative/path"`)},
		{"server.dev_directory": json.RawMessage(`"/missing/gui"`)},
	} {
		if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: patch}); err == nil {
			t.Fatalf("accepted invalid server patch: %v", patch)
		}
		if !reflect.DeepEqual(document, guiConfigDocument(t, path)) {
			t.Fatal("invalid server patch changed config")
		}
	}
	if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{"server.dev_mode": json.RawMessage("false"), "server.dev_directory": json.RawMessage(`""`)}}); err != nil {
		t.Fatal(err)
	}
	mode, source, err = config.ResolveServerDevelopment("dev", directory)
	if err != nil || mode != "normal" || source != "" {
		t.Fatal("disabled preference did not control restart")
	}
}

func TestGUIFileChangesPreferencePersists(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		raw := json.RawMessage(`false`)
		if enabled {
			raw = json.RawMessage(`true`)
		}
		gui, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{"show_file_changes": raw}})
		if err != nil {
			t.Fatal(err)
		}
		if gui.Advanced["show_file_changes"] != enabled {
			t.Fatalf("preference: %+v", gui.Advanced)
		}
		loaded, err := config.Load()
		if err != nil || loaded.FileChangesEnabled() != enabled {
			t.Fatalf("persistence: %v %+v", err, loaded)
		}
	}
	if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{"show_file_changes": json.RawMessage(`"true"`)}}); err == nil {
		t.Fatal("accepted non-boolean")
	}
}
