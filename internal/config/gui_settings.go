package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
	toml "github.com/pelletier/go-toml/v2"
)

// GUISettings follows the sections in the Settings page. Provider credentials
// and runtime model selection remain in their existing backend sections.
type GUISettings struct {
	Background GUIBackgroundSettings `toml:"background" json:"background"`
	General    GUIGeneralSettings    `toml:"-" json:"general"`
	Advanced   map[string]any        `toml:"-" json:"advanced"`
	Chat       GUIChatSettings       `toml:"chat" json:"chat"`
	Models     GUIModelSettings      `toml:"models" json:"models"`
	Docs       struct{}              `toml:"docs" json:"docs"`
}

type GUIBackgroundSettings struct {
	KeepAwake bool `toml:"keep_awake" json:"keepAwake"`
}

func UpdateGUIBackgroundSettings(enabled bool) error {
	_, err := updateGUISettings(func(gui *GUISettings, _ map[string]any) error {
		gui.Background.KeepAwake = enabled
		return nil
	})
	return err
}

// General settings expose runtime preferences without duplicating their TOML storage.
type GUIGeneralSettings struct {
	UserName         string `json:"userName"`
	ResponseLanguage string `json:"responseLanguage"`
}

type GUIGeneralPatch struct {
	UserName         *string `json:"userName"`
	ResponseLanguage *string `json:"responseLanguage"`
}

type GUISettingsPatch struct {
	Advanced map[string]json.RawMessage `json:"advanced"`
	General  *GUIGeneralPatch           `json:"general"`
	Chat     GUIChatSettings            `json:"chat"`
}

func (patch GUISettingsPatch) Validate() error {
	if patch.General != nil {
		if patch.General.UserName == nil && patch.General.ResponseLanguage == nil {
			return fmt.Errorf("at least one general preference is required")
		}
		for name, value := range map[string]*string{"user name": patch.General.UserName, "response language": patch.General.ResponseLanguage} {
			if value == nil {
				continue
			}
			text := strings.TrimSpace(*value)
			if text == "" || len(text) > 256 || strings.ContainsAny(text, "\r\n\x00") {
				return fmt.Errorf("%s requires a nonempty single-line string of at most 256 bytes", name)
			}
		}
	}
	if patch.Chat.FloatingLettersDensity != nil {
		switch *patch.Chat.FloatingLettersDensity {
		case "low", "medium", "high":
		default:
			return fmt.Errorf("floating letters density must be low, medium or high")
		}
	}
	if err := validateGUIAdvancedSettings(patch.Advanced); err != nil {
		return err
	}
	if patch.General == nil && len(patch.Advanced) == 0 && patch.Chat.AutoCloseToolCalls == nil && patch.Chat.StartToolCallsCollapsed == nil && patch.Chat.StartReasoningCollapsed == nil && patch.Chat.FloatingLetters == nil && patch.Chat.FloatingLettersDensity == nil {
		return fmt.Errorf("at least one preference is required")
	}
	return nil
}

type GUIChatSettings struct {
	FloatingLettersDensity  *string `toml:"floating_letters_density,omitempty" json:"floatingLettersDensity,omitempty"`
	FloatingLetters         *bool   `toml:"floating_letters,omitempty" json:"floatingLetters,omitempty"`
	AutoCloseToolCalls      *bool   `toml:"auto_close_tool_calls,omitempty" json:"autoCloseToolCalls,omitempty"`
	StartToolCallsCollapsed *bool   `toml:"start_tool_calls_collapsed,omitempty" json:"startToolCallsCollapsed,omitempty"`
	StartReasoningCollapsed *bool   `toml:"start_reasoning_collapsed,omitempty" json:"startReasoningCollapsed,omitempty"`
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
	var runtime Root
	if err := toml.Unmarshal(data, &runtime); err != nil {
		return GUISettings{}, err
	}
	file.GUI.General = GUIGeneralSettings{UserName: runtime.UserName, ResponseLanguage: runtime.EffectiveResponseLanguage()}
	file.GUI.Advanced = guiAdvancedValues(&runtime)
	return file.GUI, nil
}

// updateGUISettings applies targeted preferences and preserves unknown backend
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
	return UpdateGUIPreferences(GUISettingsPatch{Chat: patch})
}

func UpdateGUIPreferences(patch GUISettingsPatch) (GUISettings, error) {
	if err := patch.Validate(); err != nil {
		return GUISettings{}, err
	}
	return updateGUISettings(func(gui *GUISettings, document map[string]any) error {
		if patch.General != nil {
			if patch.General.ResponseLanguage != nil {
				language := strings.TrimSpace(*patch.General.ResponseLanguage)
				document["response_language"] = language
				gui.General.ResponseLanguage = language
			}
			if patch.General.UserName != nil {
				name := strings.TrimSpace(*patch.General.UserName)
				document["user_name"] = name
				gui.General.UserName = name
			}
		}
		for key, raw := range patch.Advanced {
			var value any
			if key == "server.dev_mode" {
				var enabled bool
				_ = json.Unmarshal(raw, &enabled)
				value = enabled
			} else if key == "server.dev_directory" {
				var directory string
				_ = json.Unmarshal(raw, &directory)
				directory = strings.TrimSpace(directory)

				value = directory
			} else if key == "log_level" {
				var level string
				_ = json.Unmarshal(raw, &level)
				value = strings.ToLower(strings.TrimSpace(level))
			} else {
				var number int64
				_ = json.Unmarshal(raw, &number)
				value = number
			}
			if strings.Contains(key, ".") {
				section, field, _ := strings.Cut(key, ".")
				subtree, _ := document[section].(map[string]any)
				if subtree == nil {
					subtree = map[string]any{}
				}
				subtree[field] = value
				document[section] = subtree
			} else {
				document[key] = value
			}
			gui.Advanced[key] = value
		}
		_, modeChanged := patch.Advanced["server.dev_mode"]
		_, directoryChanged := patch.Advanced["server.dev_directory"]
		if (modeChanged || directoryChanged) && gui.Advanced["server.dev_mode"] == true {
			if _, err := ValidateDevDirectory(gui.Advanced["server.dev_directory"].(string)); err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidServerDevelopment, err)
			}
		}
		if patch.Chat.AutoCloseToolCalls != nil {
			gui.Chat.AutoCloseToolCalls = patch.Chat.AutoCloseToolCalls
		}
		if patch.Chat.StartToolCallsCollapsed != nil {
			gui.Chat.StartToolCallsCollapsed = patch.Chat.StartToolCallsCollapsed
		}
		if patch.Chat.FloatingLettersDensity != nil {
			gui.Chat.FloatingLettersDensity = patch.Chat.FloatingLettersDensity
		}
		if patch.Chat.FloatingLetters != nil {
			gui.Chat.FloatingLetters = patch.Chat.FloatingLetters
		}
		if patch.Chat.StartReasoningCollapsed != nil {
			gui.Chat.StartReasoningCollapsed = patch.Chat.StartReasoningCollapsed
		}
		return nil
	})
}
