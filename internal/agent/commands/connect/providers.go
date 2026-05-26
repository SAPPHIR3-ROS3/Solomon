package connect

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

func readLine(d Deps, prompt string) (string, error) {
	return config.ReadPromptLine(PromptIO(d), prompt)
}

func chooseKind(d Deps) (int, error) {
	printSystem(d, strings.Join([]string{
		"Connect provider type:",
		"  1) ChatGPT Sub (browser sign-in)",
		"  2) OpenAI Compatible API (base URL + API key)",
		"  3) Anthropic Compatible API (base URL + API key)",
		"  4) Claude Sub (OAuth, coming soon)",
		"  5) Cursor API (API key)",
	}, "\n"))
	line, err := readLine(d, "Select [1-5]: ")
	if err != nil {
		return 0, err
	}
	if line == "" {
		return 0, fmt.Errorf("missing selection")
	}
	switch line {
	case "1":
		return 1, nil
	case "2":
		return 2, nil
	case "3":
		return 3, nil
	case "4":
		return 4, nil
	case "5":
		return 5, nil
	default:
		return 0, fmt.Errorf("invalid selection %q (use 1, 2, 3, 4, or 5)", line)
	}
}

func chatGPTSub(d Deps) error {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	tokens, err := codex.Login(ctx, d.Out)
	if err != nil {
		return err
	}
	prov, err := config.NewChatGPTSubProvider(codex.ChatGPTSubAPIBase, tokens)
	if err != nil {
		return err
	}
	prevProv := d.Cfg.Current.Provider
	prevModel := d.Cfg.Current.Model
	config.AppendOrUpdateProvider(d.Cfg, prov)
	if err := d.SaveCfg(); err != nil {
		return err
	}
	ids := codex.SubModelCatalog()
	return pickModel(d, prevProv, prevModel, prov.Name, ids)
}

func claudeSubComingSoon(d Deps) error {
	printSystem(d, strings.Join([]string{
		"Claude Sub (OAuth via Agent SDK) is not available yet.",
		fmt.Sprintf("Expected after %s when Anthropic enables subscription auth for third-party apps.", config.ClaudeSubExpectedDate),
		"Use option 3 (Anthropic API key) until then.",
	}, "\n"))
	return nil
}

func compatibleAPI(d Deps) error {
	n, err := readLine(d, "Provider display name: ")
	if err != nil {
		return err
	}
	if n == "" {
		return fmt.Errorf("missing provider display name")
	}
	if err := rejectReservedProviderName(n); err != nil {
		return err
	}
	u, err := readLine(d, "Base URL: ")
	if err != nil {
		return err
	}
	if u == "" {
		return fmt.Errorf("missing base URL")
	}
	k, err := readLine(d, "API key: ")
	if err != nil {
		return err
	}
	if k == "" {
		return fmt.Errorf("missing API key")
	}
	base, err := config.NormalizeAPIBase(u)
	if err != nil {
		return err
	}
	prov := config.Provider{Name: n, BaseURL: base, APIKey: k, AuthKind: config.AuthKindAPIKey, APIProtocol: config.APIProtocolOpenAI}
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ids, err := ListModelsForProvider(ctx, d.Cfg, &prov)
	if err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}
	prevProv := d.Cfg.Current.Provider
	prevModel := d.Cfg.Current.Model
	config.AppendOrUpdateProvider(d.Cfg, prov)
	if err := d.SaveCfg(); err != nil {
		return err
	}
	return pickModel(d, prevProv, prevModel, prov.Name, ids)
}

func anthropicCompatibleAPI(d Deps) error {
	n, err := readLine(d, "Provider display name: ")
	if err != nil {
		return err
	}
	if n == "" {
		return fmt.Errorf("missing provider display name")
	}
	if err := rejectReservedProviderName(n); err != nil {
		return err
	}
	u, err := readLine(d, "Base URL e.g. https://api.anthropic.com: ")
	if err != nil {
		return err
	}
	if u == "" {
		return fmt.Errorf("missing base URL")
	}
	k, err := readLine(d, "API key: ")
	if err != nil {
		return err
	}
	if k == "" {
		return fmt.Errorf("missing API key")
	}
	base, err := config.NormalizeAnthropicBase(u)
	if err != nil {
		return err
	}
	prov := config.Provider{Name: n, BaseURL: base, APIKey: k, AuthKind: config.AuthKindAPIKey, APIProtocol: config.APIProtocolAnthropic}
	prevProv := d.Cfg.Current.Provider
	prevModel := d.Cfg.Current.Model
	config.AppendOrUpdateProvider(d.Cfg, prov)
	if err := d.SaveCfg(); err != nil {
		return err
	}
	ids := modelsapi.CuratedAnthropicModels()
	return pickModel(d, prevProv, prevModel, prov.Name, ids)
}

func rejectReservedProviderName(n string) error {
	switch n {
	case config.ProviderNameChatGPTSub:
		return fmt.Errorf("display name %q is reserved; use option 1 for ChatGPT Sub", n)
	case config.ProviderNameClaudeSub:
		return fmt.Errorf("display name %q is reserved; use option 4 for Claude Sub", n)
	case config.ProviderNameCursorAPI:
		return fmt.Errorf("display name %q is reserved; use option 5 for Cursor API", n)
	default:
		return nil
	}
}

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
		return nil, err
	}
	if p.IsCursorAPI() {
		return cursorint.OrderModelIDs(ids), nil
	}
	return ids, err
}
