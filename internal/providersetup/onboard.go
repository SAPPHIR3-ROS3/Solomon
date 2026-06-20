package providersetup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func RunOnboardProviderSetup(pio config.PromptIO, existing *config.Root, opts config.OnboardOpts, res *config.OnboardResult) error {
	out := pio.Out
	if out == nil {
		out = os.Stdout
	}
	kind, skipped, err := config.ChooseProviderKind(pio, opts.RequireProvider, "LLM provider type:")
	if err != nil {
		return err
	}
	if skipped {
		config.PrintConfigSkipHint(out, "provider")
		return config.ErrOnboardProviderSkipped
	}
	setupRes, err := RunProviderSetupByKind(context.Background(), pio, nil, existing, kind, config.ProviderSetupOpts{
		RequireProvider: opts.RequireProvider,
		WriteToConfig:   false,
	})
	if err != nil {
		if errors.Is(err, config.ErrOnboardProviderSkipped) {
			config.PrintConfigSkipHint(out, "provider")
		} else {
			logging.Log(logging.ERROR_LOG_LEVEL, "onboard provider setup failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
		return err
	}
	if setupRes == nil {
		return nil
	}
	res.NewProvider = &setupRes.Provider
	if setupRes.SwitchCurrent {
		res.SwitchCurrent = true
		res.CurrentProvider = setupRes.CurrentProvider
		res.CurrentModel = setupRes.CurrentModel
	}
	return nil
}

func RunOnboardWizard(pio config.PromptIO, existing *config.Root, opts config.OnboardOpts) (*config.OnboardResult, error) {
	res, err := config.RunOnboardWizard(pio, existing, opts)
	if err != nil {
		return nil, err
	}
	if err := RunOnboardProviderSetup(pio, existing, opts, res); err != nil {
		if errors.Is(err, config.ErrOnboardProviderSkipped) {
			return res, nil
		}
		return nil, err
	}
	return res, nil
}

func RunInitialSetup(pio config.PromptIO, errOut io.Writer, cfg *config.Root, configExists bool) error {
	out := pio.Out
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	if !config.NeedsOnboard(cfg) {
		return nil
	}
	if !configExists {
		fmt.Fprintln(out, "Welcome to Solomon. Set up your LLM provider to get started.")
		fmt.Fprintln(out)
	} else {
		fmt.Fprintln(out, "LLM setup incomplete. Let's finish configuration.")
		fmt.Fprintln(out)
	}
	opts := config.OnboardOpts{RequireProvider: true}
	for config.NeedsOnboard(cfg) {
		res, err := RunOnboardWizard(pio, cfg, opts)
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "initial setup wizard failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			fmt.Fprintf(errOut, "%v\n", err)
			if strings.Contains(err.Error(), "unexpected end of input") {
				return err
			}
			fmt.Fprintln(out, "Setup failed. Please try again.")
			fmt.Fprintln(out)
			continue
		}
		config.ApplyOnboardMerge(cfg, res)
		if config.NeedsOnboard(cfg) {
			fmt.Fprintln(out, "Provider, API key, and model are required.")
			fmt.Fprintln(out)
		}
	}
	return config.Save(cfg)
}
