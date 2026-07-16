package config

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"

func TableCharacteristics(cfg *Root) []string {
	if cfg == nil {
		return nil
	}
	return append([]string(nil), cfg.Roles.Table.Characteristics...)
}

func HasRolesTable(cfg *Root) bool {
	return cfg != nil && len(cfg.Roles.Table.Characteristics) > 0
}

func RolesSubagentEntries(cfg *Root) []roles.SubagentEntry {
	if cfg == nil {
		return nil
	}
	out := make([]roles.SubagentEntry, 0, len(cfg.Roles.Subagent))
	for _, e := range cfg.Roles.Subagent {
		scores := map[string]int{}
		for k, v := range e.Scores {
			scores[k] = v
		}
		out = append(out, roles.SubagentEntry{
			Provider:    e.Provider,
			Model:       e.Model,
			Description: e.Description,
			Scores:      scores,
		})
	}
	return out
}

func RolesScoreWarnings(r *Root) []string {
	if r == nil {
		return nil
	}
	return roles.OrphanScoreWarnings(r.Roles.Table.Characteristics, RolesSubagentEntries(r))
}
