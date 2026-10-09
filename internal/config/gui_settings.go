package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
	toml "github.com/pelletier/go-toml/v2"
)

// GUISettings follows the sections in the Settings page. Provider credentials
// and runtime model selection remain in their existing backend sections.
type GUISettings struct {
	Chat   GUIChatSettings  `toml:"chat" json:"chat"`
	Models GUIModelSettings `toml:"models" json:"models"`
	Docs   struct{}         `toml:"docs" json:"docs"`
}

type GUIChatSettings struct {
	AutoCloseToolCalls      *bool `toml:"auto_close_tool_calls,omitempty" json:"autoCloseToolCalls,omitempty"`
	StartToolCallsCollapsed *bool `toml:"start_tool_calls_collapsed,omitempty" json:"startToolCallsCollapsed,omitempty"`
}

type GUIModelSettings struct {
	HiddenModels map[string][]string `toml:"hidden_models" json:"hiddenModels"`
}

func guiFingerprint(gui GUISettings) string {
	data, _ := json.Marshal(gui)
	return string(data)
}

// ReadGUISettings avoids role validation and provider requests. Missing chat
// values are left unset so the GUI can migrate its old browser preferences.
func ReadGUISettings() (GUISettings, error) {
	path, err := paths.ConfigPath()
	if err != nil {
		return GUISettings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return GUISettings{}, err
	}
	return guiSettingsFromBytes(data)
}

func guiSettingsFromBytes(data []byte) (GUISettings, error) {
	var file struct {
		GUI          GUISettings         `toml:"gui"`
		HiddenModels map[string][]string `toml:"hidden_models"`
	}
	if err := toml.Unmarshal(data, &file); err != nil {
		return GUISettings{}, err
	}
	var sections struct {
		GUI map[string]any `toml:"gui"`
	}
	if err := toml.Unmarshal(data, &sections); err != nil {
		return GUISettings{}, err
	}
	_, modelsConfigured := sections.GUI["models"]
	if file.GUI.Models.HiddenModels == nil && !modelsConfigured {
		file.GUI.Models.HiddenModels = file.HiddenModels
		home, err := paths.SolomonHome()
		if err != nil {
			return GUISettings{}, err
		}
		legacy, err := os.ReadFile(filepath.Join(home, "model-visibility.json"))
		if err == nil {
			if err := json.Unmarshal(legacy, &file.GUI.Models.HiddenModels); err != nil {
				return GUISettings{}, fmt.Errorf("read legacy model visibility: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return GUISettings{}, err
		}
	}
	if file.GUI.Models.HiddenModels == nil {
		file.GUI.Models.HiddenModels = map[string][]string{}
	}
	return file.GUI, nil
}

// updateGUISettings changes only the GUI subtree and preserves unknown backend
// keys. It shares the lock with other targeted config writers.
func updateGUISettings(mutate func(*GUISettings, map[string]any) error) (GUISettings, error) {
	path, err := paths.ConfigPath()
	if err != nil {
		return GUISettings{}, err
	}
	lock := flock.New(filepath.Join(filepath.Dir(path), "config.lock"))
	if err := lock.Lock(); err != nil {
		return GUISettings{}, err
	}
	defer lock.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return GUISettings{}, err
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return GUISettings{}, err
	}
	gui, err := guiSettingsFromBytes(data)
	if err != nil {
		return GUISettings{}, err
	}
	if err := mutate(&gui, document); err != nil {
		return GUISettings{}, err
	}
	encoded, err := toml.Marshal(gui)
	if err != nil {
		return GUISettings{}, err
	}
	var subtree map[string]any
	if err := toml.Unmarshal(encoded, &subtree); err != nil {
		return GUISettings{}, err
	}
	// Keep empty model preferences explicit so migrated sidecar values cannot
	// reappear after the last hidden model is enabled.
	models := subtree["models"].(map[string]any)
	hidden := map[string]any{}
	for provider, ids := range gui.Models.HiddenModels {
		hidden[provider] = ids
	}
	models["hidden_models"] = hidden
	// Preserve future GUI fields when a client updates a known preference.
	existing, _ := document["gui"].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	for section, value := range subtree {
		fields, ok := value.(map[string]any)
		current, _ := existing[section].(map[string]any)
		if ok && current != nil {
			for key, field := range fields {
				current[key] = field
			}
		} else {
			existing[section] = value
		}
	}
	document["gui"] = existing
	delete(document, "hidden_models")
	encoded, err = toml.Marshal(document)
	if err != nil {
		return GUISettings{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".gui-config-*")
	if err != nil {
		return GUISettings{}, err
	}
	defer os.Remove(temporary.Name())
	_, writeErr := temporary.Write(encoded)
	closeErr := temporary.Close()
	if writeErr != nil {
		return GUISettings{}, writeErr
	}
	if closeErr != nil {
		return GUISettings{}, closeErr
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return GUISettings{}, err
	}
	return gui, nil
}

func UpdateGUIChatSettings(patch GUIChatSettings) (GUISettings, error) {
	if patch.AutoCloseToolCalls == nil && patch.StartToolCallsCollapsed == nil {
		return GUISettings{}, fmt.Errorf("at least one chat preference is required")
	}
	return updateGUISettings(func(gui *GUISettings, _ map[string]any) error {
		if patch.AutoCloseToolCalls != nil {
			gui.Chat.AutoCloseToolCalls = patch.AutoCloseToolCalls
		}
		if patch.StartToolCallsCollapsed != nil {
			gui.Chat.StartToolCallsCollapsed = patch.StartToolCallsCollapsed
		}
		return nil
	})
}
