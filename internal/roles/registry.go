package roles

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type SubagentEntry struct {
	Provider    string
	Model       string
	Description string
	Points      int
}

func SubagentPool(cfg *config.Root) []SubagentEntry {
	if cfg == nil {
		return nil
	}
	out := make([]SubagentEntry, 0, len(cfg.Roles.Subagent))
	for _, e := range cfg.Roles.Subagent {
		p := strings.TrimSpace(e.Provider)
		mod := strings.TrimSpace(e.Model)
		if p == "" || mod == "" {
			continue
		}
		out = append(out, SubagentEntry{
			Provider:    p,
			Model:       mod,
			Description: strings.TrimSpace(e.Description),
			Points:      e.EffectivePoints(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Points != out[j].Points {
			return out[i].Points > out[j].Points
		}
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Model < out[j].Model
	})
	return out
}

func FindSubagent(cfg *config.Root, provider, model string) (SubagentEntry, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider == "" || model == "" {
		return SubagentEntry{}, fmt.Errorf("subagent role provider and model are required together")
	}
	for _, e := range SubagentPool(cfg) {
		if e.Provider == provider && e.Model == model {
			return e, nil
		}
	}
	return SubagentEntry{}, fmt.Errorf("subagent role not found for provider %q model %q (use listSubAgents)", provider, model)
}
