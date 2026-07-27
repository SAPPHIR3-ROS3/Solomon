package config

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

type SubagentAddResult struct {
	Provider    string
	Model       string
	Description string
	Scores      map[string]int
}

func CompleteSubagentEntry(ctx context.Context, pio PromptIO, cfg *Root, provider, modelID string) (*SubagentAddResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config unavailable")
	}
	provider = strings.TrimSpace(provider)
	modelID = strings.TrimSpace(modelID)
	if provider == "" || modelID == "" {
		return nil, fmt.Errorf("provider and model are required")
	}
	if len(cfg.Roles.Table.Characteristics) == 0 {
		return nil, fmt.Errorf("roles table not configured")
	}
	out := pio.promptOut()
	cache := map[string][]string{}
	if err := validateSubagentRoleStruct(cfg, 0, SubagentRoleConfig{Provider: provider, Model: modelID}); err != nil {
		return nil, err
	}
	ids, err := cachedProviderModels(ctx, cfg, provider, cache)
	if err != nil {
		return nil, fmt.Errorf("provider %q: %w", provider, err)
	}
	if !modelInProviderList(ids, modelID) {
		return nil, fmt.Errorf("model %q not in provider %q model list", modelID, provider)
	}
	for _, e := range cfg.Roles.Subagent {
		if subagentRoleKey(e.Provider, e.Model) == subagentRoleKey(provider, modelID) {
			return nil, fmt.Errorf("subagent already configured for %q %q", provider, modelID)
		}
	}
	descLine, err := readOnboardLine(pio, "Description (optional): ")
	if err != nil {
		return nil, err
	}
	scores := map[string]int{}
	for _, ch := range cfg.Roles.Table.Characteristics {
		for {
			line, err := readOnboardLine(pio, fmt.Sprintf("Score for %s (0-100): ", roles.CharacteristicColumn(ch)))
			if err != nil {
				return nil, err
			}
			v, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil {
				fmt.Fprintln(out, "Enter an integer 0-100.")
				continue
			}
			if err := roles.ValidateScoreValue(ch, v); err != nil {
				fmt.Fprintln(out, err.Error())
				continue
			}
			scores[ch] = v
			break
		}
	}
	return &SubagentAddResult{
		Provider:    provider,
		Model:       modelID,
		Description: strings.TrimSpace(descLine),
		Scores:      scores,
	}, nil
}

func ApplySubagentAdd(cfg *Root, res *SubagentAddResult) {
	if cfg == nil || res == nil {
		return
	}
	entry := SubagentRoleConfig{
		Provider:    res.Provider,
		Model:       res.Model,
		Description: res.Description,
	}
	if len(res.Scores) > 0 {
		entry.Scores = map[string]int{}
		for k, v := range res.Scores {
			entry.Scores[k] = v
		}
	}
	cfg.Roles.Subagent = append(cfg.Roles.Subagent, entry)
}
