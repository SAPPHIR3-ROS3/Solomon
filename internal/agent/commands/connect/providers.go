package connect

import (
	"context"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

func ListModelsForProvider(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
	if p.IsClaudeSub() {
		return modelsapi.CuratedClaudeSubModels(), nil
	}
	if p.EffectiveAuthKind() == config.AuthKindOAuthChatGPT {
		return codex.SubModelCatalog(), nil
	}
	if p.IsAnthropic() {
		return modelsapi.CuratedAnthropicModels(), nil
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
		return modelsapi.CuratedClaudeSubModels(), nil
	}
	if p.EffectiveAuthKind() == config.AuthKindOAuthChatGPT {
		return codex.SubModelCatalog(), nil
	}
	if p.IsAnthropic() {
		return modelsapi.CuratedAnthropicModels(), nil
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
