package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
)

var errOnboardProviderSkipped = errors.New("onboard provider skipped")

const (
	onboardAPIOpenAI     = 1
	onboardAPIAnthropic  = 2
)

func onboardChooseAPIKind(pio PromptIO, require bool) (kind int, skipped bool, err error) {
	out := pio.promptOut()
	fmt.Fprintln(out, "LLM provider API type:")
	fmt.Fprintln(out, "  1) OpenAI Compatible API (base URL + API key)")
	fmt.Fprintln(out, "  2) Anthropic Compatible API (base URL + API key)")
	prompt := "Select [1-2]: "
	if !require {
		prompt = "Select [1-2] (skip to skip provider setup): "
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
			return onboardAPIOpenAI, false, nil
		case "2":
			return onboardAPIAnthropic, false, nil
		default:
			fmt.Fprintln(out, "Invalid selection (use 1 or 2).")
		}
	}
}

func onboardListModels(p *Provider) ([]string, error) {
	if p.IsAnthropic() {
		return modelsapi.CuratedAnthropicModels(), nil
	}
	ids, err := modelsapi.List(p.BaseURL, p.APIKey)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no models returned by API")
	}
	return ids, nil
}

func runOnboardProviderSetup(pio PromptIO, existing *Root, opts OnboardOpts, res *OnboardResult) error {
	out := pio.promptOut()
	apiKind, skipped, err := onboardChooseAPIKind(pio, opts.RequireProvider)
	if err != nil {
		return err
	}
	if skipped {
		PrintConfigSkipHint(out, "provider")
		return errOnboardProviderSkipped
	}
	setupTitle := "Solomon setup: OpenAI Compatible API"
	basePrompt := "Base URL e.g. https://api.openai.com"
	if apiKind == onboardAPIAnthropic {
		setupTitle = "Solomon setup: Anthropic Compatible API"
		basePrompt = "Base URL e.g. https://api.anthropic.com"
	}
	fmt.Fprintln(out, setupTitle)
	var provNameLine string
	if opts.RequireProvider {
		provNameLine, err = readRequired(pio, "Display name for this provider: ")
		if err != nil {
			return err
		}
	} else {
		provNameLine, err = readOnboardLine(pio, "Display name for this provider (skip to skip provider setup): ")
		if err != nil {
			return err
		}
		if isSkipInput(provNameLine) {
			PrintConfigSkipHint(out, "provider")
			return errOnboardProviderSkipped
		}
		if provNameLine == "" {
			return fmt.Errorf("provider display name cannot be empty (type skip to skip provider setup)")
		}
	}
	var base, key string
	if opts.RequireProvider {
		base, err = readRequired(pio, basePrompt+": ")
		if err != nil {
			return err
		}
		key, err = readRequired(pio, "API key: ")
		if err != nil {
			return err
		}
	} else {
		var baseSkipped bool
		base, baseSkipped, err = readRequiredOrSkip(pio, basePrompt+" (skip to skip provider setup): ")
		if err != nil {
			return err
		}
		if baseSkipped {
			PrintConfigSkipHint(out, "provider")
			return errOnboardProviderSkipped
		}
		key, baseSkipped, err = readRequiredOrSkip(pio, "API key (skip to skip provider setup): ")
		if err != nil {
			return err
		}
		if baseSkipped {
			PrintConfigSkipHint(out, "provider")
			return errOnboardProviderSkipped
		}
	}
	p := Provider{Name: provNameLine, APIKey: key, AuthKind: AuthKindAPIKey}
	if apiKind == onboardAPIAnthropic {
		p.APIProtocol = APIProtocolAnthropic
		norm, normErr := NormalizeAnthropicBase(base)
		if normErr != nil {
			return normErr
		}
		p.BaseURL = norm
	} else {
		p.APIProtocol = APIProtocolOpenAI
		norm, normErr := NormalizeAPIBase(base)
		if normErr != nil {
			return normErr
		}
		p.BaseURL = norm
	}
	if apiKind == onboardAPIAnthropic {
		fmt.Fprint(out, "Using curated Anthropic model list…\n")
	} else {
		fmt.Fprint(out, "Fetching models…\n")
	}
	ids, err := onboardListModels(&p)
	if err != nil {
		return err
	}
	res.NewProvider = &p
	hadExisting := existing != nil && len(existing.Providers) > 0 && strings.TrimSpace(existing.Current.Model) != ""
	if hadExisting {
		choice, err := PickModelAfterAdd(pio, existing.Current.Provider, existing.Current.Model, p.Name, ids, !opts.RequireProvider)
		if err != nil {
			return err
		}
		if choice.Changed {
			res.SwitchCurrent = true
			res.CurrentProvider = choice.ProviderName
			res.CurrentModel = choice.ModelID
		}
	} else {
		mid, err := PickModelInteractive(pio, &p, p.Name, ids, !opts.RequireProvider)
		if err != nil {
			return err
		}
		res.SwitchCurrent = true
		res.CurrentProvider = p.Name
		res.CurrentModel = mid
	}
	return nil
}

func EmptyRoot() *Root {
	return &Root{
		SubagentTimeoutMinutes:    DefaultSubagentTimeoutMinutes,
		CompactionThresholdTokens: DefaultCompactionThresholdTokens,
	}
}

func ConfigExists() (bool, error) {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(cfgPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func LoadOptional() (*Root, error) {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(cfgPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return EmptyRoot(), nil
		}
		return nil, statErr
	}
	return Load()
}

func NeedsOnboard(r *Root) bool {
	if r == nil || len(r.Providers) == 0 {
		return true
	}
	if strings.TrimSpace(r.Current.Model) == "" {
		return true
	}
	p := lookupProvider(r, r.Current.Provider)
	if p == nil {
		return true
	}
	if strings.TrimSpace(p.BaseURL) == "" {
		return true
	}
	if !ProviderCredentialsReady(p) {
		return true
	}
	return false
}

func lookupProvider(r *Root, name string) *Provider {
	if r == nil || len(r.Providers) == 0 {
		return nil
	}
	if p := ProviderByName(r, name); p != nil {
		return p
	}
	first := FirstProviderName(r)
	if first == "" {
		return nil
	}
	return ProviderByName(r, first)
}

func WriteConfigSetupWarning(w io.Writer, r *Root) {
	if w == nil || !NeedsOnboard(r) {
		return
	}
	var missing []string
	if len(r.Providers) == 0 {
		missing = append(missing, "providers")
	} else {
		if strings.TrimSpace(r.Current.Model) == "" {
			missing = append(missing, "current.model")
		}
		if strings.TrimSpace(r.Current.Provider) == "" {
			missing = append(missing, "current.provider")
		}
		p := lookupProvider(r, r.Current.Provider)
		if p == nil {
			missing = append(missing, "providers")
		} else {
			if strings.TrimSpace(p.Name) == "" {
				missing = append(missing, "providers[].name")
			}
			if strings.TrimSpace(p.BaseURL) == "" {
				missing = append(missing, "providers[].base_url")
			}
			if !ProviderCredentialsReady(p) {
				if p.IsOAuthProvider() {
					missing = append(missing, "providers[].oauth tokens")
				} else {
					missing = append(missing, "providers[].api_key")
				}
			}
		}
	}
	fmt.Fprintf(w, "warning: LLM setup incomplete (%s); use /onboard or edit ~/.solomon/config.toml\n", strings.Join(missing, ", "))
}

func PrintConfigSkipHint(out io.Writer, topic string) {
	if out == nil {
		out = os.Stdout
	}
	switch topic {
	case "user_name":
		fmt.Fprintln(out, "Skipped. Configure later: /name <your name>  —  or in ~/.solomon/config.toml: user_name = \"...\"")
	case "provider":
		fmt.Fprintln(out, "Skipped. Configure later: /onboard or /connect  —  or in ~/.solomon/config.toml:")
		fmt.Fprintln(out, "  [providers.my-provider]")
		fmt.Fprintln(out, "  base_url = \"https://...\"")
		fmt.Fprintln(out, "  api_key = \"...\"")
		fmt.Fprintln(out, "  api_protocol = \"openai\"  # or \"anthropic\"")
		fmt.Fprintln(out, "  [current]")
		fmt.Fprintln(out, "  provider = \"...\"")
		fmt.Fprintln(out, "  model = \"...\"")
	case "response_language":
		fmt.Fprintf(out, "Skipped. Configure later: /language <language>  —  or in ~/.solomon/config.toml: response_language = \"%s\"\n", DefaultResponseLanguage)
	case "current_model":
		fmt.Fprintln(out, "Skipped. Configure later: /models  —  or in ~/.solomon/config.toml under [current]: provider = \"...\" and model = \"...\"")
	default:
		fmt.Fprintln(out, "Skipped. Configure later: /onboard or edit ~/.solomon/config.toml")
	}
}

type OnboardResult struct {
	UserName                  string
	ResponseLanguage          string
	SubagentTimeoutMinutes    int
	CompactionThresholdTokens int64
	NewProvider               *Provider
	SwitchCurrent             bool
	CurrentProvider           string
	CurrentModel              string
}

func ApplyOnboardMerge(dst *Root, res *OnboardResult) {
	if dst == nil || res == nil {
		return
	}
	dst.UserName = res.UserName
	dst.ResponseLanguage = res.ResponseLanguage
	dst.SubagentTimeoutMinutes = res.SubagentTimeoutMinutes
	dst.CompactionThresholdTokens = res.CompactionThresholdTokens
	if res.NewProvider != nil {
		AppendOrUpdateProvider(dst, *res.NewProvider)
	}
	if res.SwitchCurrent {
		dst.Current = Current{Provider: res.CurrentProvider, Model: res.CurrentModel}
	}
}

func isSkipInput(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "skip")
}

func readOnboardLine(pio PromptIO, prompt string) (string, error) {
	line, err := ReadPromptLine(pio, prompt)
	if err != nil {
		if err == io.EOF {
			return "", fmt.Errorf("unexpected end of input")
		}
		return "", err
	}
	return line, nil
}

func readRequiredOrSkip(pio PromptIO, prompt string) (value string, skipped bool, err error) {
	out := pio.promptOut()
	for {
		line, err := readOnboardLine(pio, prompt)
		if err != nil {
			return "", false, err
		}
		if isSkipInput(line) {
			return "", true, nil
		}
		if line == "" {
			fmt.Fprintln(out, "Required: enter a value or type skip.")
			continue
		}
		return line, false, nil
	}
}

func readRequired(pio PromptIO, prompt string) (string, error) {
	out := pio.promptOut()
	for {
		line, err := readOnboardLine(pio, prompt)
		if err != nil {
			return "", err
		}
		if line == "" {
			fmt.Fprintln(out, "Required: enter a value.")
			continue
		}
		return line, nil
	}
}

type OnboardOpts struct {
	RequireProvider bool
}

func RunOnboardWizard(pio PromptIO, existing *Root, opts OnboardOpts) (*OnboardResult, error) {
	out := pio.promptOut()
	res := &OnboardResult{
		SubagentTimeoutMinutes:    DefaultSubagentTimeoutMinutes,
		CompactionThresholdTokens: DefaultCompactionThresholdTokens,
		ResponseLanguage:          DefaultResponseLanguage,
	}
	nameLine, err := readOnboardLine(pio, "Your name (skip for default): ")
	if err != nil {
		return nil, err
	}
	if isSkipInput(nameLine) {
		PrintConfigSkipHint(out, "user_name")
	} else {
		res.UserName = nameLine
	}
	if err := runOnboardProviderSetup(pio, existing, opts, res); err != nil {
		if err == errOnboardProviderSkipped {
			goto language
		}
		return nil, err
	}
language:
	langLine, err := readOnboardLine(pio, fmt.Sprintf("Assistant response language [%s] (skip for default): ", DefaultResponseLanguage))
	if err != nil {
		return nil, err
	}
	if isSkipInput(langLine) {
		PrintConfigSkipHint(out, "response_language")
	} else if langLine != "" {
		res.ResponseLanguage = langLine
	}
	return res, nil
}

func ConfirmOnboardRerun(pio PromptIO) (bool, error) {
	line, err := ReadPromptLine(pio, "Re-run onboarding will update profile fields and may add a provider. Continue? [y/N]: ")
	if err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	switch strings.ToLower(line) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
