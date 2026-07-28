package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type desktopModelChoice struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type desktopProviderCatalog struct {
	Complete bool     `json:"complete"`
	Models   []string `json:"models"`
	Provider string   `json:"provider"`
}

type desktopModelCatalog struct {
	Current   desktopModelChoice       `json:"current"`
	Providers []desktopProviderCatalog `json:"providers"`
	Recent    []desktopModelChoice     `json:"recent"`
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

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
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
			if provider.Name == cfg.Current.Provider {
				ids = ensureDesktopModelFirst(ids, cfg.Current.Model)
			}
			result.Providers[index] = desktopProviderCatalog{
				Provider: provider.Name,
				Models:   ids,
				Complete: complete,
			}
		}()
	}
	wg.Wait()
	return result
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
