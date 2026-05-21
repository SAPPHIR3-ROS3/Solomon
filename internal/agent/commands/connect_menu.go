package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

func connectChooseKind(d Deps, sc *bufio.Scanner) (int, error) {
	fmt.Fprintln(d.Out, "Connect provider type:")
	fmt.Fprintln(d.Out, "  1) ChatGPT Sub (browser sign-in)")
	fmt.Fprintln(d.Out, "  2) OpenAI Compatible API (base URL + API key)")
	fmt.Fprintln(d.Out, "  3) Anthropic Compatible API (base URL + API key)")
	fmt.Fprint(d.Out, "Select [1-3]: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("missing selection")
	}
	line := strings.TrimSpace(sc.Text())
	switch line {
	case "1":
		return 1, nil
	case "2":
		return 2, nil
	case "3":
		return 3, nil
	default:
		return 0, fmt.Errorf("invalid selection %q (use 1, 2, or 3)", line)
	}
}

func connectChatGPTSub(d Deps, sc *bufio.Scanner) error {
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
	return connectPickModel(d, sc, prevProv, prevModel, prov.Name, ids)
}

func connectCompatibleAPI(d Deps, sc *bufio.Scanner) error {
	fmt.Fprint(d.Out, "Provider display name: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing provider display name")
	}
	n := strings.TrimSpace(sc.Text())
	if n == config.ProviderNameChatGPTSub {
		return fmt.Errorf("display name %q is reserved; use option 1 for ChatGPT Sub", n)
	}
	fmt.Fprint(d.Out, "Base URL: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing base URL")
	}
	u := strings.TrimSpace(sc.Text())
	fmt.Fprint(d.Out, "API key: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing API key")
	}
	k := strings.TrimSpace(sc.Text())
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
	return connectPickModel(d, sc, prevProv, prevModel, prov.Name, ids)
}

func connectAnthropicCompatibleAPI(d Deps, sc *bufio.Scanner) error {
	fmt.Fprint(d.Out, "Provider display name: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing provider display name")
	}
	n := strings.TrimSpace(sc.Text())
	if n == config.ProviderNameChatGPTSub {
		return fmt.Errorf("display name %q is reserved; use option 1 for ChatGPT Sub", n)
	}
	fmt.Fprint(d.Out, "Base URL e.g. https://api.anthropic.com: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing base URL")
	}
	u := strings.TrimSpace(sc.Text())
	fmt.Fprint(d.Out, "API key: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing API key")
	}
	k := strings.TrimSpace(sc.Text())
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
	return connectPickModel(d, sc, prevProv, prevModel, prov.Name, ids)
}

func listModelsForProvider(ctx context.Context, cfg *config.Root, p *config.Provider) ([]string, error) {
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

func connectScanner(stdin io.Reader) *bufio.Scanner {
	if stdin == nil {
		stdin = os.Stdin
	}
	return bufio.NewScanner(stdin)
}
