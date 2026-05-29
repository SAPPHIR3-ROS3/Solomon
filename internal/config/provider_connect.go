package config

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

const (
	ProviderKindChatGPTSub          = 1
	ProviderKindOpenAICompatible    = 2
	ProviderKindAnthropicCompatible = 3
	ProviderKindClaudeSub           = 4
	ProviderKindCursorAPI           = 5
)

func ProviderConnectMenuLines() []string {
	return []string{
		"  1) ChatGPT Sub (browser sign-in)",
		"  2) OpenAI Compatible API (base URL + API key)",
		"  3) Anthropic Compatible API (base URL + API key)",
		"  4) Claude Sub (OAuth, coming soon)",
		"  5) Cursor API (API key)",
	}
}

func PrintProviderConnectMenu(out io.Writer, title string) {
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintln(out, title)
	for _, line := range ProviderConnectMenuLines() {
		fmt.Fprintln(out, line)
	}
}

func ChooseProviderKind(pio PromptIO, require bool, menuTitle string) (kind int, skipped bool, err error) {
	out := pio.promptOut()
	if strings.TrimSpace(menuTitle) == "" {
		menuTitle = "LLM provider type:"
	}
	PrintProviderConnectMenu(out, menuTitle)
	prompt := "Select [1-5]: "
	if !require {
		prompt = "Select [1-5] (skip to skip provider setup): "
	}
	for {
		line, err := readOnboardLine(pio, prompt)
		if err != nil {
			return 0, false, err
		}
		if !require && isSkipInput(line) {
			return 0, true, nil
		}
		switch strings.TrimSpace(line) {
		case "1":
			return ProviderKindChatGPTSub, false, nil
		case "2":
			return ProviderKindOpenAICompatible, false, nil
		case "3":
			return ProviderKindAnthropicCompatible, false, nil
		case "4":
			return ProviderKindClaudeSub, false, nil
		case "5":
			return ProviderKindCursorAPI, false, nil
		default:
			fmt.Fprintln(out, "Invalid selection (use 1, 2, 3, 4, or 5).")
		}
	}
}

type ProviderSetupOpts struct {
	RequireProvider bool
	WriteToConfig   bool
	SaveConfig      func() error
}

type ProviderSetupResult struct {
	Provider        Provider
	SwitchCurrent   bool
	CurrentProvider string
	CurrentModel    string
}

func RunProviderSetupByKind(ctx context.Context, pio PromptIO, cfg *Root, existing *Root, kind int, opts ProviderSetupOpts) (*ProviderSetupResult, error) {
	switch kind {
	case ProviderKindChatGPTSub:
		return setupChatGPTSub(ctx, pio, cfg, existing, opts)
	case ProviderKindOpenAICompatible:
		return setupOpenAICompatibleAPI(ctx, pio, cfg, existing, opts)
	case ProviderKindAnthropicCompatible:
		return setupAnthropicCompatibleAPI(pio, cfg, existing, opts)
	case ProviderKindClaudeSub:
		return nil, printClaudeSubComingSoon(pio)
	default:
		return nil, fmt.Errorf("internal error: unknown provider kind %d", kind)
	}
}

func printClaudeSubComingSoon(pio PromptIO) error {
	out := pio.promptOut()
	fmt.Fprintln(out, strings.Join([]string{
		"Claude Sub (OAuth via Agent SDK) is not available yet.",
		fmt.Sprintf("Expected after %s when Anthropic enables subscription auth for third-party apps.", ClaudeSubExpectedDate),
		"Use option 3 (Anthropic API key) until then.",
	}, "\n"))
	return nil
}

func setupChatGPTSub(ctx context.Context, pio PromptIO, cfg *Root, existing *Root, opts ProviderSetupOpts) (*ProviderSetupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	tokens, err := codex.Login(ctx, pio.promptOut())
	if err != nil {
		return nil, err
	}
	prov, err := NewChatGPTSubProvider(codex.ChatGPTSubAPIBase, tokens)
	if err != nil {
		return nil, err
	}
	ids := codex.SubModelCatalog()
	return FinalizeProviderSetup(pio, cfg, existing, opts, prov, ids)
}

func setupOpenAICompatibleAPI(ctx context.Context, pio PromptIO, cfg *Root, existing *Root, opts ProviderSetupOpts) (*ProviderSetupResult, error) {
	out := pio.promptOut()
	fmt.Fprintln(out, "Solomon setup: OpenAI Compatible API")
	prov, ids, err := readCompatibleAPIProvider(pio, opts, APIProtocolOpenAI, "https://api.openai.com")
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ids, err = listModelsForNewProvider(pio, &prov, ids)
	if err != nil {
		return nil, err
	}
	return FinalizeProviderSetup(pio, cfg, existing, opts, prov, ids)
}

func setupAnthropicCompatibleAPI(pio PromptIO, cfg *Root, existing *Root, opts ProviderSetupOpts) (*ProviderSetupResult, error) {
	out := pio.promptOut()
	fmt.Fprintln(out, "Solomon setup: Anthropic Compatible API")
	prov, ids, err := readCompatibleAPIProvider(pio, opts, APIProtocolAnthropic, "https://api.anthropic.com")
	if err != nil {
		return nil, err
	}
	fmt.Fprint(out, "Using curated Anthropic model list…\n")
	return FinalizeProviderSetup(pio, cfg, existing, opts, prov, ids)
}

func readCompatibleAPIProvider(pio PromptIO, opts ProviderSetupOpts, protocol, defaultBaseHint string) (Provider, []string, error) {
	var provNameLine string
	var err error
	if opts.RequireProvider {
		provNameLine, err = readRequired(pio, "Display name for this provider: ")
	} else {
		provNameLine, err = readOnboardLine(pio, "Display name for this provider (skip to skip provider setup): ")
	}
	if err != nil {
		return Provider{}, nil, err
	}
	if !opts.RequireProvider && isSkipInput(provNameLine) {
		return Provider{}, nil, ErrOnboardProviderSkipped
	}
	if provNameLine == "" {
		return Provider{}, nil, fmt.Errorf("provider display name cannot be empty (type skip to skip provider setup)")
	}
	if err := rejectReservedProviderName(provNameLine); err != nil {
		return Provider{}, nil, err
	}
	basePrompt := "Base URL e.g. " + defaultBaseHint
	var base, key string
	if opts.RequireProvider {
		base, err = readRequired(pio, basePrompt+": ")
		if err != nil {
			return Provider{}, nil, err
		}
		key, err = readRequired(pio, "API key: ")
		if err != nil {
			return Provider{}, nil, err
		}
	} else {
		var skipped bool
		base, skipped, err = readRequiredOrSkip(pio, basePrompt+" (skip to skip provider setup): ")
		if err != nil {
			return Provider{}, nil, err
		}
		if skipped {
			return Provider{}, nil, ErrOnboardProviderSkipped
		}
		key, skipped, err = readRequiredOrSkip(pio, "API key (skip to skip provider setup): ")
		if err != nil {
			return Provider{}, nil, err
		}
		if skipped {
			return Provider{}, nil, ErrOnboardProviderSkipped
		}
	}
	p := Provider{Name: provNameLine, APIKey: key, AuthKind: AuthKindAPIKey}
	if protocol == APIProtocolAnthropic {
		p.APIProtocol = APIProtocolAnthropic
		norm, normErr := NormalizeAnthropicBase(base)
		if normErr != nil {
			return Provider{}, nil, normErr
		}
		p.BaseURL = norm
		return p, modelsapi.CuratedAnthropicModels(), nil
	}
	p.APIProtocol = APIProtocolOpenAI
	norm, normErr := NormalizeAPIBase(base)
	if normErr != nil {
		return Provider{}, nil, normErr
	}
	p.BaseURL = norm
	return p, nil, nil
}

func listModelsForNewProvider(pio PromptIO, p *Provider, curated []string) ([]string, error) {
	if len(curated) > 0 {
		return curated, nil
	}
	fmt.Fprint(pio.promptOut(), "Fetching models…\n")
	ids, err := modelsapi.List(p.BaseURL, p.APIKey)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no models returned by API")
	}
	return ids, nil
}

func FinalizeProviderSetup(pio PromptIO, cfg *Root, existing *Root, opts ProviderSetupOpts, prov Provider, ids []string) (*ProviderSetupResult, error) {
	res := &ProviderSetupResult{Provider: prov}
	if opts.WriteToConfig && cfg != nil {
		AppendOrUpdateProvider(cfg, prov)
		if opts.SaveConfig != nil {
			if err := opts.SaveConfig(); err != nil {
				return nil, err
			}
		}
	}
	allowSkip := !opts.RequireProvider
	prevProv, prevModel := "", ""
	if existing != nil {
		prevProv = existing.Current.Provider
		prevModel = existing.Current.Model
	}
	hadExisting := existing != nil && len(existing.Providers) > 0 && strings.TrimSpace(existing.Current.Model) != ""
	if hadExisting {
		choice, err := PickModelAfterAdd(pio, prevProv, prevModel, prov.Name, ids, allowSkip)
		if err != nil {
			return nil, err
		}
		if choice.Changed {
			res.SwitchCurrent = true
			res.CurrentProvider = choice.ProviderName
			res.CurrentModel = choice.ModelID
		}
		return res, nil
	}
	mid, err := PickModelInteractive(pio, &prov, prov.Name, ids, allowSkip)
	if err != nil {
		return nil, err
	}
	res.SwitchCurrent = true
	res.CurrentProvider = prov.Name
	res.CurrentModel = mid
	return res, nil
}

func rejectReservedProviderName(n string) error {
	switch n {
	case ProviderNameChatGPTSub:
		return fmt.Errorf("display name %q is reserved; use option 1 for ChatGPT Sub", n)
	case ProviderNameClaudeSub:
		return fmt.Errorf("display name %q is reserved; use option 4 for Claude Sub", n)
	case ProviderNameCursorAPI:
		return fmt.Errorf("display name %q is reserved; use option 5 for Cursor API", n)
	default:
		return nil
	}
}
