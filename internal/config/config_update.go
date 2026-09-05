package config

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
)

// UpdateCurrentModel atomically persists the selected provider and model.
func UpdateCurrentModel(providerName, modelID string) (*Root, error) {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return nil, fmt.Errorf("provider and model are required")
	}
	return updateConfig(func(root *Root) error {
		if ProviderByName(root, providerName) == nil {
			return fmt.Errorf("unknown provider %q", providerName)
		}
		changed := root.Current.Provider != providerName || root.Current.Model != modelID
		root.Current.Provider = providerName
		root.Current.Model = modelID
		if changed {
			NoteRecentModelUse(root, providerName, modelID)
		}
		return nil
	})
}

// UpdateReasoningEffort atomically persists the main-chat reasoning level.
func UpdateReasoningEffort(effort string) (*Root, error) {
	canonical, err := ParseReasoningEffortToken(effort)
	if err != nil {
		return nil, err
	}
	return updateConfig(func(root *Root) error {
		root.ReasoningEffort = canonical
		return nil
	})
}

func updateConfig(mutate func(*Root) error) (*Root, error) {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(filepath.Dir(cfgPath), "config.lock"))
	if err := lock.Lock(); err != nil {
		return nil, err
	}
	defer lock.Unlock()

	root, err := Load()
	if err != nil {
		return nil, err
	}
	if err := mutate(root); err != nil {
		return nil, err
	}
	normalizeRoot(root)
	if err := validateRoot(context.Background(), root); err != nil {
		return nil, err
	}
	if err := Save(root); err != nil {
		return nil, err
	}
	return root, nil
}
