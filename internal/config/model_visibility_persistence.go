package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
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
	lock := flock.New(filepath.Join(filepath.Dir(cfgPath), "model-visibility.lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
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

	buf, err := json.Marshal(file.HiddenModels)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return err
	}
	target := filepath.Join(filepath.Dir(cfgPath), "model-visibility.json")
	fileHandle, err := os.CreateTemp(filepath.Dir(cfgPath), ".model-visibility-*")
	if err != nil {
		return err
	}
	defer os.Remove(fileHandle.Name())
	_, writeErr := fileHandle.Write(buf)
	closeErr := fileHandle.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(fileHandle.Name(), target); err != nil {
		return err
	}
	return nil
}

func updateModelVisibilityRoot(root *Root, providerName, modelID string, enabled bool) error {
	if ProviderByName(root, providerName) == nil {
		return fmt.Errorf("unknown provider %q", providerName)
	}
	if err := LoadModelVisibility(root); err != nil {
		return err
	}
	return SetModelEnabled(root, providerName, modelID, enabled)
}

// LoadModelVisibility overlays dedicated preferences on legacy hidden_models.
// Without the dedicated file, existing config.toml preferences remain intact.
func LoadModelVisibility(root *Root) error {
	if root == nil {
		return nil
	}
	home, err := paths.SolomonHome()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(home, "model-visibility.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var hidden map[string][]string
	if err := json.Unmarshal(data, &hidden); err != nil {
		return fmt.Errorf("read model visibility: %w", err)
	}
	root.HiddenModels = hidden
	return nil
}

// ReadModelVisibility reads preferences without provider calls or role validation.
func ReadModelVisibility() (*Root, error) {
	path, err := paths.ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var legacy struct {
		HiddenModels map[string][]string `toml:"hidden_models"`
	}
	if err := to.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	root := &Root{HiddenModels: legacy.HiddenModels}
	if err := LoadModelVisibility(root); err != nil {
		return nil, err
	}
	return root, nil
}
