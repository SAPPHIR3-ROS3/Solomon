package config

import (
	"context"
	"fmt"
	"strings"
)

var RolesModelLister func(context.Context, *Root, *Provider) ([]string, error)

const DefaultSubagentRolePoints = 100

type Roles struct {
	Subagent []SubagentRoleConfig `toml:"subagent,omitempty"`
}

type SubagentRoleConfig struct {
	Provider    string `toml:"provider"`
	Model       string `toml:"model"`
	Description string `toml:"description,omitempty"`
	Points      int    `toml:"points,omitempty"`
}

func (e SubagentRoleConfig) EffectivePoints() int {
	if e.Points > 0 {
		return e.Points
	}
	return DefaultSubagentRolePoints
}

func subagentRoleKey(provider, model string) string {
	return strings.TrimSpace(provider) + "\x00" + strings.TrimSpace(model)
}

func validateSubagentRoleStruct(r *Root, index int, e SubagentRoleConfig) error {
	provider := strings.TrimSpace(e.Provider)
	model := strings.TrimSpace(e.Model)
	if provider == "" {
		return fmt.Errorf("roles.subagent[%d]: missing provider", index)
	}
	if model == "" {
		return fmt.Errorf("roles.subagent[%d]: missing model", index)
	}
	if e.Points < 0 {
		return fmt.Errorf("roles.subagent[%d]: points must be >= 0", index)
	}
	if ProviderByName(r, provider) == nil {
		return fmt.Errorf("roles.subagent[%d]: provider %q not found in config", index, provider)
	}
	return nil
}

func modelInProviderList(ids []string, modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	for _, id := range ids {
		if strings.TrimSpace(id) == modelID {
			return true
		}
	}
	return false
}

func cachedProviderModels(ctx context.Context, r *Root, providerName string, cache map[string][]string) ([]string, error) {
	if RolesModelLister == nil {
		return nil, fmt.Errorf("roles model lister not configured")
	}
	if ids, ok := cache[providerName]; ok {
		return ids, nil
	}
	p := ProviderByName(r, providerName)
	if p == nil {
		return nil, fmt.Errorf("provider not found in config")
	}
	ids, err := RolesModelLister(ctx, r, p)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("provider returned no models")
	}
	cache[providerName] = ids
	return ids, nil
}

func ValidateRoles(ctx context.Context, r *Root) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	seen := map[string]int{}
	modelCache := map[string][]string{}
	for i, e := range r.Roles.Subagent {
		if err := validateSubagentRoleStruct(r, i, e); err != nil {
			return err
		}
		key := subagentRoleKey(e.Provider, e.Model)
		if prev, ok := seen[key]; ok {
			return fmt.Errorf("roles.subagent[%d]: duplicate provider %q model %q (also at [%d])", i, strings.TrimSpace(e.Provider), strings.TrimSpace(e.Model), prev)
		}
		seen[key] = i
		provider := strings.TrimSpace(e.Provider)
		model := strings.TrimSpace(e.Model)
		ids, err := cachedProviderModels(ctx, r, provider, modelCache)
		if err != nil {
			return fmt.Errorf("roles.subagent[%d]: provider %q: %w", i, provider, err)
		}
		if !modelInProviderList(ids, model) {
			return fmt.Errorf("roles.subagent[%d]: model %q not in provider %q model list", i, model, provider)
		}
	}
	return nil
}
