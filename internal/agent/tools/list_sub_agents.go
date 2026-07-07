package tools

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureListSubAgents() {}

func listSubAgentsSummary() string {
	return "List configured subagent roles from [[roles.subagent]] in config.toml. Compare description and points, then pass the chosen provider and model to subagent."
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
	entries := roles.SubagentPool(env.Cfg)
	type row struct {
		Provider    string `json:"provider"`
		Model       string `json:"model"`
		Description string `json:"description,omitempty"`
		Points      int    `json:"points"`
	}
	out := make([]row, 0, len(entries))
	for _, e := range entries {
		out = append(out, row{
			Provider:    e.Provider,
			Model:       e.Model,
			Description: e.Description,
			Points:      e.Points,
		})
	}
	return map[string]any{
		"subagents": out,
		"count":     len(out),
		"hint":      "Pass provider and model from the chosen row to subagent.",
	}, nil
}
