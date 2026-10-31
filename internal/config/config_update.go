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
		return AssignCurrentModel(root, providerName, modelID)
	})
}

// AssignCurrentModel stores the outgoing model's mode and restores the incoming one on root.
func AssignCurrentModel(root *Root, providerName, modelID string) error {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return fmt.Errorf("provider and model are required")
	}
	if root == nil {
		return fmt.Errorf("config not loaded")
	}
	if ProviderByName(root, providerName) == nil {
		return fmt.Errorf("unknown provider %q", providerName)
	}
	changed := root.Current.Provider != providerName || root.Current.Model != modelID
	if changed {
		RememberCurrentModelMode(root)
	}
	root.Current.Provider = providerName
	root.Current.Model = modelID
	if changed {
		NoteRecentModelUse(root, providerName, modelID)
		applyModelMode(root, providerName, modelID)
	}
	capability := ModelReasoning(ProviderByName(root, providerName), modelID)
	if capability.Known {
		root.ReasoningEffort = capability.Resolve(root.ReasoningEffort)
		RememberCurrentModelMode(root)
	}
	return nil
}

// UpdateReasoningEffort atomically persists the main-chat reasoning level.
func UpdateReasoningEffort(effort string) (*Root, error) {
	canonical, err := ParseReasoningEffortToken(effort)
	if err != nil {
		return nil, err
	}
	return updateConfig(func(root *Root) error {
		if err := validateModelReasoning(ProviderByName(root, root.Current.Provider), root.Current.Model, canonical); err != nil {
			return err
		}
		root.ReasoningEffort = canonical
		RememberCurrentModelMode(root)
		return nil
	})
}

// UpdateFastMode atomically persists fast mode for the current provider model.
func UpdateFastMode(enabled bool) (*Root, error) {
	return updateConfig(func(root *Root) error {
		root.FastMode = &enabled
		RememberCurrentModelMode(root)
		return nil
	})
}

// RememberCurrentModelMode stores reasoning and fast mode for the current provider model.
func RememberCurrentModelMode(root *Root) {
	if root == nil {
		return
	}
	providerName := strings.TrimSpace(root.Current.Provider)
	modelID := strings.TrimSpace(root.Current.Model)
	if providerName == "" || modelID == "" {
		return
	}
	if root.ModelModes == nil {
		root.ModelModes = map[string]map[string]ModelMode{}
	}
	modes := root.ModelModes[providerName]
	if modes == nil {
		modes = map[string]ModelMode{}
		root.ModelModes[providerName] = modes
	}
	fast := root.EffectiveFastMode()
	modes[modelID] = ModelMode{ReasoningEffort: root.ReasoningEffort, FastMode: &fast}
}

func applyModelMode(root *Root, providerName, modelID string) {
	modes := root.ModelModes[providerName]
	mode, ok := modes[modelID]
	if !ok {
		RememberCurrentModelMode(root)
		return
	}
	if mode.ReasoningEffort == "" {
		root.ReasoningEffort = ""
	} else if canonical, err := ParseReasoningEffortToken(mode.ReasoningEffort); err == nil {
		root.ReasoningEffort = canonical
	}
	if mode.FastMode != nil {
		fast := *mode.FastMode
		root.FastMode = &fast
	}
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
	if err := saveUnlocked(root); err != nil {
		return nil, err
	}
	return root, nil
}

// UpdateProvider merges a connection into the latest config after browser login.
func UpdateProvider(provider Provider) (*Root, error) {
	return updateConfig(func(root *Root) error { AppendOrUpdateProvider(root, provider); return nil })
}
