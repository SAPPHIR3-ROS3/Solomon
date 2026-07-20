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
)

type modelChoice struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type providerCatalog struct {
	Provider string   `json:"provider"`
	Models   []string `json:"models"`
	Complete bool     `json:"complete"`
}

type catalogResponse struct {
	Current   modelChoice       `json:"current"`
	Recent    []modelChoice     `json:"recent"`
	Providers []providerCatalog `json:"providers"`
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

func main() {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	_ = logging.Configure(logging.Config{WriteConsole: false, WriteFile: false})
	config.RolesModelLister = connect.ListModelsForProviderAll
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	configuredProviders := config.ProviderList(cfg)
	providers := make([]config.Provider, 0, len(configuredProviders))
	for _, provider := range configuredProviders {
		if provider.Name != config.ProviderNameClaudeSub {
			providers = append(providers, provider)
		}
	}
	result := catalogResponse{
		Current: modelChoice{Provider: strings.TrimSpace(cfg.Current.Provider), Model: strings.TrimSpace(cfg.Current.Model)},
	}
	for _, entry := range config.RecentModelUseEntries(cfg, cfg.Current.Provider) {
		if strings.TrimSpace(entry.Provider) == config.ProviderNameClaudeSub {
			continue
		}
		result.Recent = append(result.Recent, modelChoice{Provider: strings.TrimSpace(entry.Provider), Model: strings.TrimSpace(entry.Model)})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	result.Providers = make([]providerCatalog, len(providers))
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
			if provider.Name == cfg.Current.Provider {
				ids = ensureModelFirst(ids, cfg.Current.Model)
			}
			result.Providers[index] = providerCatalog{Provider: provider.Name, Models: ids, Complete: complete}
			if listErr != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", provider.Name, listErr)
			}
		}()
	}
	wg.Wait()

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
