package main

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func discoverACPModels(ctx context.Context, cfg *config.Root) []string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	providers := config.ProviderList(cfg)
	type catalog struct {
		provider string
		models   []string
	}
	results := make(chan catalog, len(providers))
	for _, provider := range providers {
		go func(p config.Provider) {
			copyCfg := *cfg
			copyCfg.Providers = make(map[string]*config.Provider, len(cfg.Providers))
			for name, original := range cfg.Providers {
				if original != nil {
					cp := *original
					copyCfg.Providers[name] = &cp
				}
			}
			ids, err := connect.ListModelsForProviderAll(ctx, &copyCfg, &p)
			if err != nil {
				ids = nil
			}
			results <- catalog{provider: p.Name, models: ids}
		}(provider)
	}
	seen := map[string]bool{}
	for remaining := len(providers); remaining > 0; remaining-- {
		select {
		case <-ctx.Done():
			return sortedACPModels(seen)
		case result := <-results:
			for _, id := range result.models {
				if strings.TrimSpace(id) != "" {
					seen[result.provider+"/"+id] = true
				}
			}
		}
	}
	return sortedACPModels(seen)
}
func sortedACPModels(seen map[string]bool) []string {
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
