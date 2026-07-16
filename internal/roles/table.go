package roles

import (
	"fmt"
	"sort"
	"strings"
)

type SubagentEntry struct {
	Provider    string
	Model       string
	Description string
	Scores      map[string]int
}

type TableView struct {
	Columns []string
	Rows    []TableRow
}

type TableRow struct {
	Provider     string
	Model        string
	Description  string
	Scores       map[string]int
	Unclassified bool
}

func SubagentPool(entries []SubagentEntry) []SubagentEntry {
	out := make([]SubagentEntry, 0, len(entries))
	for _, e := range entries {
		p := strings.TrimSpace(e.Provider)
		mod := strings.TrimSpace(e.Model)
		if p == "" || mod == "" {
			continue
		}
		scores := map[string]int{}
		for k, v := range e.Scores {
			scores[k] = v
		}
		out = append(out, SubagentEntry{
			Provider:    p,
			Model:       mod,
			Description: strings.TrimSpace(e.Description),
			Scores:      scores,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Model < out[j].Model
	})
	return out
}

func FindSubagent(entries []SubagentEntry, provider, model string) (SubagentEntry, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider == "" || model == "" {
		return SubagentEntry{}, fmt.Errorf("subagent role provider and model are required together")
	}
	for _, e := range SubagentPool(entries) {
		if e.Provider == provider && e.Model == model {
			return e, nil
		}
	}
	return SubagentEntry{}, fmt.Errorf("subagent role not found for provider %q model %q (use listSubAgents)", provider, model)
}

func BuildManualTableView(columns []string, entries []SubagentEntry) (TableView, error) {
	if len(columns) == 0 {
		return TableView{}, fmt.Errorf("roles table not configured")
	}
	pool := SubagentPool(entries)
	rows := make([]TableRow, 0, len(pool))
	for _, e := range pool {
		scores := map[string]int{}
		unclassified := false
		for _, ch := range columns {
			if v, ok := e.Scores[ch]; ok {
				if err := ValidateScoreValue(ch, v); err != nil {
					return TableView{}, err
				}
				scores[ch] = v
			} else {
				unclassified = true
			}
		}
		rows = append(rows, TableRow{
			Provider:     e.Provider,
			Model:        e.Model,
			Description:  e.Description,
			Scores:       scores,
			Unclassified: unclassified,
		})
	}
	return TableView{Columns: columns, Rows: rows}, nil
}

func OrphanScoreWarnings(tableCols []string, entries []SubagentEntry) []string {
	table := map[string]bool{}
	for _, ch := range tableCols {
		table[ch] = true
	}
	var warns []string
	for i, e := range entries {
		for ch := range e.Scores {
			if !table[ch] {
				warns = append(warns, fmt.Sprintf("roles.subagent[%d].scores.%s: not in roles.table; change table to use this override", i, ch))
			}
		}
	}
	return warns
}
