package connect

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

func listAnthropicModels(p *config.Provider, bearer string) ([]string, error) {
	if p == nil {
		return nil, fmt.Errorf("provider is nil")
	}
	ids, err := modelsapi.ListAnthropic(p.BaseURL, bearer, p.UsesAnthropicOAuthBearer())
	if err != nil {
		return nil, err
	}
	return modelsapi.PickAnthropicFlagshipModels(ids), nil
}

func listChatGPTSubModels(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
	bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
	if err != nil {
		return nil, err
	}
	accountID := ""
	if p != nil {
		accountID = strings.TrimSpace(p.OAuthAccountID)
	}
	ids, err := codex.ListModels(ctx, bearer, accountID)
	if err != nil {
		return nil, err
	}
	return filterChatGPTSubModels(ids), nil
}

func listClaudeSubModels(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
	bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
	if err != nil {
		return nil, err
	}
	ids, err := modelsapi.ListAnthropic(p.BaseURL, bearer, true)
	if err != nil {
		return nil, err
	}
	return filterClaudeSubModels(ids), nil
}

func filterChatGPTSubModels(ids []string) []string {
	var out []string
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return OrderChatGPTSubModels(out)
}

func OrderChatGPTSubModels(ids []string) []string {
	if len(ids) < 2 {
		return ids
	}
	pinned := [3][]string{}
	rest := make([]string, 0, len(ids))
	for _, id := range ids {
		rank := chatGPTSubPinRank(id)
		if rank >= 0 {
			pinned[rank] = append(pinned[rank], id)
			continue
		}
		rest = append(rest, id)
	}
	out := make([]string, 0, len(ids))
	for _, group := range pinned {
		out = append(out, group...)
	}
	return append(out, rest...)
}

func chatGPTSubPinRank(id string) int {
	lower := strings.ToLower(strings.TrimSpace(id))
	switch {
	case strings.HasSuffix(lower, "-sol") || lower == "sol":
		return 0
	case strings.HasSuffix(lower, "-terra") || lower == "terra":
		return 1
	case strings.HasSuffix(lower, "-luna") || lower == "luna":
		return 2
	default:
		return -1
	}
}

func filterClaudeSubModels(ids []string) []string {
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if config.ModelPassesClaudeSubFilter(id) {
			out = append(out, id)
		}
	}
	return out
}

func ListModelsForProvider(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
	if p.IsClaudeSub() {
		ids, err := listClaudeSubModels(ctx, cfg, p)
		if err != nil {
			return nil, err
		}
		return modelsapi.PickAnthropicFlagshipModels(ids), nil
	}
	if p.EffectiveAuthKind() == config.AuthKindOAuthChatGPT {
		return listChatGPTSubModels(ctx, cfg, p)
	}
	if p.IsAnthropic() {
		bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
		if err != nil {
			return nil, err
		}
		return listAnthropicModels(p, bearer)
	}
	if p.IsCursorAPI() {
		cwd, _ := os.Getwd()
		if err := cursorint.EnsureSidecarIfConfigured(ctx, cfg, cwd, nil); err != nil {
			return nil, err
		}
	}
	bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
	if err != nil {
		return nil, err
	}
	ids, err := modelsapi.List(p.BaseURL, bearer)
	if err != nil {
		if p.IsCursorAPI() {
			return cursorint.DefaultModelIDs(), nil
		}
		return nil, err
	}
	if p.IsCursorAPI() {
		return cursorint.FilterModelIDs(ids), nil
	}
	return ids, err
}

func ListModelsForProviderAll(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
	if p.IsClaudeSub() {
		return listClaudeSubModels(ctx, cfg, p)
	}
	if p.EffectiveAuthKind() == config.AuthKindOAuthChatGPT {
		return listChatGPTSubModels(ctx, cfg, p)
	}
	if p.IsAnthropic() {
		bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
		if err != nil {
			return nil, err
		}
		ids, err := modelsapi.ListAnthropic(p.BaseURL, bearer, p.UsesAnthropicOAuthBearer())
		if err != nil {
			return nil, err
		}
		return ids, nil
	}
	if p.IsCursorAPI() {
		cwd, _ := os.Getwd()
		if err := cursorint.EnsureSidecarIfConfigured(ctx, cfg, cwd, nil); err != nil {
			return nil, err
		}
	}
	bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
	if err != nil {
		return nil, err
	}
	ids, err := modelsapi.ListWithOpts(p.BaseURL, bearer, modelsapi.ListOpts{AllModels: true})
	if err != nil {
		if p.IsCursorAPI() {
			return cursorint.DefaultModelIDs(), nil
		}
		return nil, err
	}
	if p.IsCursorAPI() {
		return cursorint.OrderModelIDs(ids), nil
	}
	return ids, err
}
