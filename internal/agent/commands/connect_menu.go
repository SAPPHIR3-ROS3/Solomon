package commands

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

func connectReadLine(d Deps, prompt string) (string, error) {
	return config.ReadPromptLine(PromptIO(d), prompt)
}

func connectChooseKind(d Deps) (int, error) {
	fmt.Fprintln(d.Out, "Connect provider type:")
	fmt.Fprintln(d.Out, "  1) ChatGPT Sub (browser sign-in)")
	fmt.Fprintln(d.Out, "  2) OpenAI Compatible API (base URL + API key)")
	fmt.Fprintln(d.Out, "  3) Anthropic Compatible API (base URL + API key)")
	fmt.Fprintln(d.Out, "  4) Claude Sub (OAuth, coming soon)")
	line, err := connectReadLine(d, "Select [1-4]: ")
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
	default:
		return 0, fmt.Errorf("invalid selection %q (use 1, 2, 3, or 4)", line)
	}
}

func connectChatGPTSub(d Deps) error {
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
	return connectPickModel(d, prevProv, prevModel, prov.Name, ids)
}

func connectClaudeSubComingSoon(d Deps) error {
	fmt.Fprintf(d.Out, "Claude Sub (OAuth via Agent SDK) is not available yet.\n")
	fmt.Fprintf(d.Out, "Expected after %s when Anthropic enables subscription auth for third-party apps.\n", config.ClaudeSubExpectedDate)
	fmt.Fprintln(d.Out, "Use option 3 (Anthropic API key) until then.")
	return nil
}

func connectCompatibleAPI(d Deps) error {
	n, err := connectReadLine(d, "Provider display name: ")
	if err != nil {
		return err
	}
	if n == "" {
		return fmt.Errorf("missing provider display name")
	}
	if n == config.ProviderNameChatGPTSub {
		return fmt.Errorf("display name %q is reserved; use option 1 for ChatGPT Sub", n)
	}
	if n == config.ProviderNameClaudeSub {
		return fmt.Errorf("display name %q is reserved; use option 4 for Claude Sub", n)
	}
	u, err := connectReadLine(d, "Base URL: ")
	if err != nil {
		return err
	}
	if u == "" {
		return fmt.Errorf("missing base URL")
	}
	k, err := connectReadLine(d, "API key: ")
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
	ids, err := listModelsForProvider(ctx, d.Cfg, &prov)
	if err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}
	prevProv := d.Cfg.Current.Provider
	prevModel := d.Cfg.Current.Model
	config.AppendOrUpdateProvider(d.Cfg, prov)
	if err := d.SaveCfg(); err != nil {
		return err
	}
	return connectPickModel(d, prevProv, prevModel, prov.Name, ids)
}

func connectAnthropicCompatibleAPI(d Deps) error {
	n, err := connectReadLine(d, "Provider display name: ")
	if err != nil {
		return err
	}
	if n == "" {
		return fmt.Errorf("missing provider display name")
	}
	if n == config.ProviderNameChatGPTSub {
		return fmt.Errorf("display name %q is reserved; use option 1 for ChatGPT Sub", n)
	}
	if n == config.ProviderNameClaudeSub {
		return fmt.Errorf("display name %q is reserved; use option 4 for Claude Sub", n)
	}
	u, err := connectReadLine(d, "Base URL e.g. https://api.anthropic.com: ")
	if err != nil {
		return err
	}
	if u == "" {
		return fmt.Errorf("missing base URL")
	}
	k, err := connectReadLine(d, "API key: ")
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
	return connectPickModel(d, prevProv, prevModel, prov.Name, ids)
}

func listModelsForProvider(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
	if p.IsClaudeSub() {
		return modelsapi.CuratedClaudeSubModels(), nil
	}
	if p.EffectiveAuthKind() == config.AuthKindOAuthChatGPT {
		return codex.SubModelCatalog(), nil
	}
	if p.IsAnthropic() {
		return modelsapi.CuratedAnthropicModels(), nil
	}
	bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
	if err != nil {
		return nil, err
	}
	return modelsapi.List(p.BaseURL, bearer)
}
