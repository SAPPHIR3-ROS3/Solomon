package config

import (
	"fmt"
	"strings"
)

// UpdateModelVisibility persists presentation preferences under gui.models,
// without provider requests or validation of unrelated runtime settings.
func UpdateModelVisibility(providerName, modelID string, enabled bool) error {
	providerName, modelID = strings.TrimSpace(providerName), strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return fmt.Errorf("provider and model are required")
	}
	_, err := updateGUISettings(func(gui *GUISettings, document map[string]any) error {
		found := false
		switch providers := document["providers"].(type) {
		case map[string]any:
			_, found = providers[providerName]
		case []map[string]any:
			for _, provider := range providers {
				if provider["name"] == providerName {
					found = true
				}
			}
		case []any:
			for _, entry := range providers {
				provider, _ := entry.(map[string]any)
				if provider["name"] == providerName {
					found = true
				}
			}
		}
		if !found {
			return fmt.Errorf("unknown provider %q", providerName)
		}
		root := &Root{}
		root.HiddenModels = gui.Models.HiddenModels
		if err := SetModelEnabled(root, providerName, modelID, enabled); err != nil {
			return err
		}
		gui.Models.HiddenModels = root.HiddenModels
		return nil
	})
	return err
}

// LoadModelVisibility accepts legacy files, but an explicit gui.models table
// takes precedence so old sidecar values cannot override newer preferences.
func LoadModelVisibility(root *Root) error {
	if root == nil {
		return nil
	}
	gui, err := ReadGUISettings()
	if err != nil {
		return err
	}
	root.GUI = gui
	root.guiSnapshot = guiFingerprint(gui)
	root.HiddenModels = gui.Models.HiddenModels
	return nil
}

func ReadModelVisibility() (*Root, error) {
	gui, err := ReadGUISettings()
	if err != nil {
		return nil, err
	}
	return &Root{GUI: gui, HiddenModels: gui.Models.HiddenModels}, nil
}
