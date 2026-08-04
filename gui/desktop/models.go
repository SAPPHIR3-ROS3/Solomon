package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelcatalogcache"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsdev"
)

func configureDesktopModelLister() {
	config.RolesModelLister = func(ctx context.Context, cfg *config.Root, provider *config.Provider) ([]string, error) {
		ids, err := connect.ListModelsForProviderAll(ctx, cfg, provider)
		if err == nil && len(ids) > 0 {
			return ids, nil
		}
		fallback := append([]string{}, cfg.RecentModels[provider.Name]...)
		if cfg.Current.Provider == provider.Name {
			fallback = append(fallback, cfg.Current.Model)
		}
		for _, entry := range cfg.Roles.Subagent {
			if strings.TrimSpace(entry.Provider) == provider.Name {
				fallback = append(fallback, strings.TrimSpace(entry.Model))
			}
		}
		fallback = uniqueDesktopModels(fallback)
		if len(fallback) > 0 {
			return fallback, nil
		}
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("provider returned no models")
	}
}

type desktopModelChoice struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type desktopProviderCatalog struct {
	Complete bool                            `json:"complete"`
	Disabled []string                        `json:"disabled,omitempty"`
	Metadata map[string]desktopModelMetadata `json:"metadata"`
	Models   []string                        `json:"models"`
	Provider string                          `json:"provider"`
}

type desktopModelMetadata struct {
	Context int      `json:"context,omitempty"`
	Input   []string `json:"input,omitempty"`
	Output  int      `json:"output,omitempty"`
}

type desktopModelCatalog struct {
	Current   desktopModelChoice       `json:"current"`
	Providers []desktopProviderCatalog `json:"providers"`
	Recent    []desktopModelChoice     `json:"recent"`
}

type desktopModelVisibility struct {
	Enabled  bool   `json:"enabled"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

func (DesktopBridge) ModelCatalog() (desktopModelCatalog, error) {
	cfg, err := config.Load()
	if err != nil {
		return desktopModelCatalog{}, fmt.Errorf("read config.toml: %w", err)
	}
	return buildDesktopModelCatalog(cfg), nil
}

func (DesktopBridge) SaveCurrentModel(providerName, modelID string) (desktopModelChoice, error) {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return desktopModelChoice{}, fmt.Errorf("provider and model are required")
	}
	cfg, err := config.Load()
	if err != nil {
		return desktopModelChoice{}, fmt.Errorf("read config.toml: %w", err)
	}
	if _, ok := cfg.Providers[providerName]; !ok {
		return desktopModelChoice{}, fmt.Errorf("unknown provider %q", providerName)
	}
	changed := cfg.Current.Provider != providerName || cfg.Current.Model != modelID
	cfg.Current.Provider = providerName
	cfg.Current.Model = modelID
	if changed {
		config.NoteRecentModelUse(cfg, providerName, modelID)
	}
	if err := config.Save(cfg); err != nil {
		return desktopModelChoice{}, fmt.Errorf("save config.toml: %w", err)
	}
	return desktopModelChoice{Provider: providerName, Model: modelID}, nil
}

func (DesktopBridge) SetModelEnabled(providerName, modelID string, enabled bool) (desktopModelVisibility, error) {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return desktopModelVisibility{}, fmt.Errorf("provider and model are required")
	}
	if err := config.UpdateModelVisibility(providerName, modelID, enabled); err != nil {
		return desktopModelVisibility{}, fmt.Errorf("save model visibility: %w", err)
	}
	return desktopModelVisibility{Enabled: enabled, Model: modelID, Provider: providerName}, nil
}

func buildDesktopModelCatalog(cfg *config.Root) desktopModelCatalog {
	result := desktopModelCatalog{
		Current: desktopModelChoice{
			Provider: strings.TrimSpace(cfg.Current.Provider),
			Model:    strings.TrimSpace(cfg.Current.Model),
		},
		Recent:    make([]desktopModelChoice, 0),
		Providers: make([]desktopProviderCatalog, 0),
	}
	for _, entry := range config.RecentModelUseEntries(cfg, cfg.Current.Provider) {
		if strings.TrimSpace(entry.Provider) == config.ProviderNameClaudeSub {
			continue
		}
		result.Recent = append(result.Recent, desktopModelChoice{
			Provider: strings.TrimSpace(entry.Provider),
			Model:    strings.TrimSpace(entry.Model),
		})
	}

	configured := config.ProviderList(cfg)
	providers := make([]config.Provider, 0, len(configured))
	for _, provider := range configured {
		if provider.Name == config.ProviderNameClaudeSub {
			continue
		}
		providers = append(providers, provider)
	}
	result.Providers = make([]desktopProviderCatalog, len(providers))
	if len(providers) == 0 {
		return result
	}
	var cached []desktopProviderCatalog
	if ok, _ := modelcatalogcache.LoadToday(&cached); ok {
		result.Providers = mergeDesktopCachedProviders(providers, cached, cfg)
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	modelsCatalog, _ := modelsdev.Load(ctx)
	var wg sync.WaitGroup
	wg.Add(len(providers))
	for index := range providers {
		index := index
		provider := providers[index]
		go func() {
			defer wg.Done()
			ids, listErr := connect.ListModelsForProviderAll(ctx, cfg, &provider)
			complete := listErr == nil && len(ids) > 0
			if !complete {
				ids = cfg.RecentModels[provider.Name]
			}
			ids = uniqueDesktopModels(ids)
			if provider.Name == config.ProviderNameChatGPTSub {
				if provider.Name == cfg.Current.Provider {
					ids = ensureDesktopModelPresent(ids, cfg.Current.Model)
				}
				ids = connect.OrderChatGPTSubModels(ids)
			} else if provider.Name == cfg.Current.Provider {
				ids = ensureDesktopModelFirst(ids, cfg.Current.Model)
			}
			result.Providers[index] = desktopProviderCatalog{
				Complete: complete,
				Disabled: config.HiddenModelIDs(cfg, provider.Name, ids),
				Metadata: desktopModelsMetadata(modelsCatalog, provider, ids),
				Models:   ids,
				Provider: provider.Name,
			}
		}()
	}
	wg.Wait()
	if desktopCatalogHasCompleteProvider(result.Providers) {
		_ = modelcatalogcache.SaveToday(result.Providers)
	}
	return result
}

func mergeDesktopCachedProviders(configured []config.Provider, cached []desktopProviderCatalog, cfg *config.Root) []desktopProviderCatalog {
	byName := make(map[string]desktopProviderCatalog, len(cached))
	for _, provider := range cached {
		byName[provider.Provider] = provider
	}
	result := make([]desktopProviderCatalog, 0, len(configured))
	for _, provider := range configured {
		if saved, ok := byName[provider.Name]; ok {
			saved.Complete = false
			saved.Disabled = config.HiddenModelIDs(cfg, provider.Name, saved.Models)
			result = append(result, saved)
			continue
		}
		ids := uniqueDesktopModels(cfg.RecentModels[provider.Name])
		if provider.Name == cfg.Current.Provider {
			ids = ensureDesktopModelFirst(ids, cfg.Current.Model)
		}
		result = append(result, desktopProviderCatalog{
			Disabled: config.HiddenModelIDs(cfg, provider.Name, ids),
			Models:   ids,
			Provider: provider.Name,
		})
	}
	return result
}

func desktopCatalogHasCompleteProvider(providers []desktopProviderCatalog) bool {
	for _, provider := range providers {
		if provider.Complete {
			return true
		}
	}
	return false
}

func desktopModelsMetadata(catalog modelsdev.Catalog, provider config.Provider, ids []string) map[string]desktopModelMetadata {
	if len(catalog) == 0 {
		return nil
	}
	metadata := make(map[string]desktopModelMetadata)
	for _, id := range ids {
		model, ok := catalog.Lookup(provider.Name, provider.BaseURL, id)
		if !ok {
			continue
		}
		metadata[id] = desktopModelMetadata{Context: model.Limit.Context, Input: model.Modalities.Input, Output: model.Limit.Output}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func uniqueDesktopModels(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func ensureDesktopModelFirst(ids []string, model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ids
	}
	out := []string{model}
	for _, id := range ids {
		if id != model {
			out = append(out, id)
		}
	}
	return out
}

func ensureDesktopModelPresent(ids []string, model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ids
	}
	for _, id := range ids {
		if id == model {
			return ids
		}
	}
	return append(ids, model)
}
