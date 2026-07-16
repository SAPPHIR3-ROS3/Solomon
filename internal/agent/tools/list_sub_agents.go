package tools

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureListSubAgents() {}

func listSubAgentsSummary() string {
	return "List configured subagent roles from [[roles.subagent]] with manually assigned scores for [roles.table] characteristics. Compare the configured values, then pass provider and model to subagent."
}

func listSubAgentsOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("listSubAgents", listSubAgentsSummary(), map[string]any{}, nil)
}

func appendListSubAgentsDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureListSubAgents)
	if err != nil {
		return err
	}
	b.addBlock("listSubAgents", listSubAgentsSummary(), sig)
	return nil
}

func execListSubAgents(env *Env, raw json.RawMessage) (any, error) {
	_ = raw
	if !config.HasRolesTable(env.Cfg) {
		return map[string]any{
			"error": "roles table not configured; run /onboard or /add subagent to set [roles.table] characteristics",
			"count": 0,
		}, nil
	}
	cols := config.TableCharacteristics(env.Cfg)
	entries := config.RolesSubagentEntries(env.Cfg)
	view, err := roles.BuildManualTableView(cols, entries)
	if err != nil {
		return nil, err
	}
	type row struct {
		Provider     string         `json:"provider"`
		Model        string         `json:"model"`
		Description  string         `json:"description,omitempty"`
		Scores       map[string]int `json:"scores"`
		Unclassified bool           `json:"unclassified,omitempty"`
	}
	out := make([]row, 0, len(view.Rows))
	for _, r := range view.Rows {
		out = append(out, row{
			Provider:     r.Provider,
			Model:        r.Model,
			Description:  r.Description,
			Scores:       r.Scores,
			Unclassified: r.Unclassified,
		})
	}
	table := roles.FormatSubagentTable(view)
	return map[string]any{
		"columns": view.Columns,
		"rows":    out,
		"count":   len(out),
		"table":   table,
		"hint":    "Pass provider and model from the chosen row to subagent.",
	}, nil
}
