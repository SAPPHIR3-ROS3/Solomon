package connect

import (
	"context"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

func listAnthropicModels(p *config.Provider, bearer string) ([]string, error) {
	if p == nil {
		return modelsapi.CuratedAnthropicModels(), nil
	}
	ids, err := modelsapi.ListAnthropic(p.BaseURL, bearer, p.UsesAnthropicOAuthBearer())
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "anthropic model list failed; using curated list", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
		ids = modelsapi.CuratedAnthropicModels()
	}
	return modelsapi.PickAnthropicFlagshipModels(ids), nil
}

func ListModelsForProvider(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
	if p.IsClaudeSub() {
		return modelsapi.PickAnthropicFlagshipModels(modelsapi.CuratedAnthropicModels()), nil
	}
	if p.EffectiveAuthKind() == config.AuthKindOAuthChatGPT {
		return codex.SubModelCatalog(), nil
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
		return modelsapi.PickAnthropicFlagshipModels(modelsapi.CuratedAnthropicModels()), nil
	}
	if p.EffectiveAuthKind() == config.AuthKindOAuthChatGPT {
		return codex.SubModelCatalog(), nil
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
