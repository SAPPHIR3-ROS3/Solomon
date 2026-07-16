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
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return out
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
