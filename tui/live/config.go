package live

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const Version = 1
const MaxConfigBytes = 256 << 10

type Layout struct {
	HomeWidth      int    `json:"home_width"`
	ComposerRows   int    `json:"composer_rows"`
	MaxInputRows   int    `json:"max_input_rows"`
	SuggestionRows int    `json:"suggestion_rows"`
	Margin         int    `json:"margin"`
	Border         string `json:"border"`
}
type Config struct {
	Mouse   bool                           `json:"mouse"`
	Version int                            `json:"version"`
	Colors  map[string]string              `json:"colors"`
	Layout  Layout                         `json:"layout"`
	Text    map[string]string              `json:"text"`
	Keys    map[string]map[string][]string `json:"keys"`
}

func Default() Config {
	return Config{Version: Version, Mouse: true,
		Colors: map[string]string{"border": "#174875", "accent": "#3b8fd1", "text": "#e8e5df", "muted": "#808080", "gold": "#ffc704", "user": "#00ffff", "assistant": "#00ff00", "tool_success": "#237a52", "system": "#aa55ff", "mockup": "#061c3b"},
		Layout: Layout{HomeWidth: 96, ComposerRows: 3, MaxInputRows: 3, SuggestionRows: 5, Margin: 1, Border: "rounded"},
		Text:   map[string]string{"placeholder": "Ask anything, or /help", "gui_placeholder": "Ask Solomon anything...", "greeting": "Welcome back, ", "workspace": "Solomon", "notifications": "Notifications · 0", "tool_icon": "●", "model_picker": "Select model", "reasoning_picker": "Reasoning effort"},
		Keys:   defaultKeys(),
	}
}
func Decode(data []byte) (Config, error) {
	cfg := Default()
	if len(data) > MaxConfigBytes {
		return Config{}, fmt.Errorf("TUI configuration exceeds %d bytes", MaxConfigBytes)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("TUI configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Config{}, fmt.Errorf("TUI configuration must contain one JSON document")
	}
	if cfg.Keys == nil {
		return Config{}, fmt.Errorf("TUI keys cannot be null")
	}
	merged := defaultKeys()
	for scope, actions := range cfg.Keys {
		if merged[scope] == nil {
			merged[scope] = map[string][]string{}
		}
		for action, bindings := range actions {
			merged[scope][action] = bindings
		}
	}
	cfg.Keys = merged
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
func Path() (string, error) {
	if path := strings.TrimSpace(os.Getenv("SOLOMON_TUI_CONFIG")); path != "" {
		return filepath.Abs(path)
	}
	home := strings.TrimSpace(os.Getenv("SOLOMON_HOME"))
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(user, ".solomon")
	}
	return filepath.Join(home, "customization", "tui", "interface.json"), nil
}
func Read(path string) (Config, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxConfigBytes+1))
	if err != nil {
		return Config{}, err
	}
	return Decode(data)
}
func Export(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(Default(), "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
