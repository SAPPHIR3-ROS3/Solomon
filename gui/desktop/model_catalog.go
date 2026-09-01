//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelcatalogcache"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsdev"
)

type modelChoice struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type providerCatalog struct {
	Complete         bool                     `json:"complete"`
	SupportsFastMode bool                     `json:"supportsFastMode"`
	Disabled         []string                 `json:"disabled,omitempty"`
	Metadata         map[string]modelMetadata `json:"metadata"`
	Models           []string                 `json:"models"`
	Provider         string                   `json:"provider"`
}

type modelMetadata struct {
	Context int      `json:"context,omitempty"`
	Input   []string `json:"input,omitempty"`
	Output  int      `json:"output,omitempty"`
}

type catalogResponse struct {
	Current   modelChoice       `json:"current"`
	Providers []providerCatalog `json:"providers"`
	Recent    []modelChoice     `json:"recent"`
}

func main() {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	_ = logging.Configure(logging.Config{WriteConsole: false, WriteFile: false})
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
		fallback = uniqueModels(fallback)
		if len(fallback) > 0 {
			return fallback, nil
		}
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("provider returned no models")
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(buildCatalog(cfg)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildCatalog(cfg *config.Root) catalogResponse {
	result := catalogResponse{
		Current: modelChoice{
			Provider: strings.TrimSpace(cfg.Current.Provider),
			Model:    strings.TrimSpace(cfg.Current.Model),
		},
		Recent:    make([]modelChoice, 0),
		Providers: make([]providerCatalog, 0),
	}
	for _, entry := range config.RecentModelUseEntries(cfg, cfg.Current.Provider) {
		if strings.TrimSpace(entry.Provider) == config.ProviderNameClaudeSub {
			continue
		}
		result.Recent = append(result.Recent, modelChoice{
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
	result.Providers = make([]providerCatalog, len(providers))
	if len(providers) == 0 {
		return result
	}
	var cached []providerCatalog
	if ok, _ := modelcatalogcache.LoadToday(&cached); ok {
		result.Providers = mergeCachedProviders(providers, cached, cfg)
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
			ids = uniqueModels(ids)
			if provider.Name == config.ProviderNameChatGPTSub {
				if provider.Name == cfg.Current.Provider {
					ids = ensureModelPresent(ids, cfg.Current.Model)
				}
				ids = connect.OrderChatGPTSubModels(ids)
			} else if provider.Name == cfg.Current.Provider {
				ids = ensureModelFirst(ids, cfg.Current.Model)
			}
			result.Providers[index] = providerCatalog{
				Complete:         complete,
				SupportsFastMode: config.FastModeSupportedByProvider(&provider),
				Disabled:         config.HiddenModelIDs(cfg, provider.Name, ids),
				Metadata:         modelsMetadata(modelsCatalog, provider, ids),
				Models:           ids,
				Provider:         provider.Name,
			}
			if listErr != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", provider.Name, listErr)
			}
		}()
	}
	wg.Wait()
	if catalogHasCompleteProvider(result.Providers) {
		_ = modelcatalogcache.SaveToday(result.Providers)
	}
	return result
}

func mergeCachedProviders(configured []config.Provider, cached []providerCatalog, cfg *config.Root) []providerCatalog {
	byName := make(map[string]providerCatalog, len(cached))
	for _, provider := range cached {
		byName[provider.Provider] = provider
	}
	result := make([]providerCatalog, 0, len(configured))
	for _, provider := range configured {
		if saved, ok := byName[provider.Name]; ok {
			saved.Complete = false
			saved.SupportsFastMode = config.FastModeSupportedByProvider(&provider)
			saved.Disabled = config.HiddenModelIDs(cfg, provider.Name, saved.Models)
			result = append(result, saved)
			continue
		}
		ids := uniqueModels(cfg.RecentModels[provider.Name])
		if provider.Name == cfg.Current.Provider {
			ids = ensureModelFirst(ids, cfg.Current.Model)
		}
		result = append(result, providerCatalog{
			Disabled:         config.HiddenModelIDs(cfg, provider.Name, ids),
			SupportsFastMode: config.FastModeSupportedByProvider(&provider),
			Models:           ids,
			Provider:         provider.Name,
		})
	}
	return result
}

func catalogHasCompleteProvider(providers []providerCatalog) bool {
	for _, provider := range providers {
		if provider.Complete {
			return true
		}
	}
	return false
}

func modelsMetadata(catalog modelsdev.Catalog, provider config.Provider, ids []string) map[string]modelMetadata {
	if len(catalog) == 0 {
		return nil
	}
	metadata := make(map[string]modelMetadata)
	for _, id := range ids {
		model, ok := catalog.Lookup(provider.Name, provider.BaseURL, id)
		if !ok {
			continue
		}
		metadata[id] = modelMetadata{Context: model.Limit.Context, Input: model.Modalities.Input, Output: model.Limit.Output}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func uniqueModels(ids []string) []string {
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

func ensureModelFirst(ids []string, model string) []string {
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

func ensureModelPresent(ids []string, model string) []string {
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
