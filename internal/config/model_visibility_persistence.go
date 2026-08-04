package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	to "github.com/pelletier/go-toml/v2"
)

// UpdateModelVisibility persists only the GUI model visibility preference.
//
// This deliberately skips full config validation. Changing whether a model is
// shown in the selector must not wait for, or fail because of, a live provider
// model listing or an unrelated subagent configuration.
func UpdateModelVisibility(providerName, modelID string, enabled bool) error {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return fmt.Errorf("provider and model are required")
	}

	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	var file rootFile
	if bytes.Contains(b, []byte("[[providers]]")) {
		var legacy rootLegacyFile
		if err := to.Unmarshal(b, &legacy); err != nil {
			return err
		}
		root := rootFromLegacy(&legacy)
		if err := updateModelVisibilityRoot(root, providerName, modelID, enabled); err != nil {
			return err
		}
		file = *rootToFile(root)
	} else {
		if err := to.Unmarshal(b, &file); err != nil {
			return err
		}
		root := rootFromFile(&file)
		if len(root.Providers) == 0 {
			var legacy rootLegacyFile
			if err := to.Unmarshal(b, &legacy); err == nil && len(legacy.Providers) > 0 {
				root = rootFromLegacy(&legacy)
			}
		}
		if err := updateModelVisibilityRoot(root, providerName, modelID, enabled); err != nil {
			return err
		}
		file = *rootToFile(root)
	}

	buf, err := to.Marshal(&file)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return err
	}
	tmp := cfgPath + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, cfgPath); err != nil {
		return err
	}
	return nil
}

func updateModelVisibilityRoot(root *Root, providerName, modelID string, enabled bool) error {
	if ProviderByName(root, providerName) == nil {
		return fmt.Errorf("unknown provider %q", providerName)
	}
	return SetModelEnabled(root, providerName, modelID, enabled)
}
