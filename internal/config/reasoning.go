package config

import (
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelreasoning"
	"strings"
)

func UpdateModelReasoning(provider, model, effort string) (*Root, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider == "" || model == "" {
		return nil, fmt.Errorf("provider and model are required")
	}
	if effort != "" {
		canonical, err := ParseReasoningEffortToken(effort)
		if err != nil {
			return nil, err
		}
		effort = canonical
	}
	return updateConfig(func(root *Root) error {
		if ProviderByName(root, provider) == nil {
			return fmt.Errorf("unknown provider %q", provider)
		}
		if err := validateModelReasoning(ProviderByName(root, provider), model, effort); err != nil {
			return err
		}
		if root.ModelModes == nil {
			root.ModelModes = map[string]map[string]ModelMode{}
		}
		if root.ModelModes[provider] == nil {
			root.ModelModes[provider] = map[string]ModelMode{}
		}
		mode := root.ModelModes[provider][model]
		mode.ReasoningEffort = effort
		root.ModelModes[provider][model] = mode
		if root.Current.Provider == provider && root.Current.Model == model {
			root.ReasoningEffort = effort
		}
		return nil
	})
}

func ModelReasoning(p *Provider, model string) modelreasoning.Capability {
	if p == nil {
		return modelreasoning.Capability{}
	}
	origin := p.BaseURL
	if p.EffectiveAuthKind() == AuthKindOAuthChatGPT {
		origin = codex.ChatGPTSubAPIBase
	}
	if p.IsClaudeSub() {
		origin = "claude-sub"
	}
	if p.IsCursorSub() {
		origin = "cursor-sub"
	}
	return modelreasoning.Lookup(origin, model)
}

func validateModelReasoning(p *Provider, model, effort string) error {
	capability := ModelReasoning(p, model)
	if effort != "" && capability.Known && capability.Resolve(effort) != effort {
		return fmt.Errorf("reasoning level %q is not supported by model %q", effort, model)
	}
	return nil
}
