package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func Onboard(d Deps) error {
	if d.Cfg == nil {
		return fmt.Errorf("/onboard unavailable")
	}
	pio := PromptIO(d)
	exists, err := config.ConfigExists()
	if err != nil {
		return err
	}
	if exists {
		ok, err := config.ConfirmOnboardRerun(pio)
		if err != nil {
			return err
		}
		if !ok {
			PrintSystem(d.Out, "Onboard cancelled.")
			return nil
		}
	}
	existing := cloneRootSnapshot(d.Cfg)
	res, err := config.RunOnboardWizard(pio, existing, config.OnboardOpts{})
	if err != nil {
		return err
	}
	return finishOnboard(d, res)
}

func finishOnboard(d Deps, res *config.OnboardResult) error {
	config.ApplyOnboardMerge(d.Cfg, res)
	if d.SaveCfg == nil {
		return fmt.Errorf("/onboard unavailable")
	}
	if err := d.SaveCfg(); err != nil {
		return err
	}
	if res.SwitchCurrent && d.ApplyCurrentModel != nil {
		if err := d.ApplyCurrentModel(res.CurrentProvider, res.CurrentModel); err != nil {
			return err
		}
	} else if d.Provider() == nil && !config.NeedsOnboard(d.Cfg) && d.ApplyCurrentModel != nil {
		if err := d.ApplyCurrentModel(d.Cfg.Current.Provider, d.Cfg.Current.Model); err != nil {
			return err
		}
	}
	if d.SetCompactionThresholdTokens != nil {
		d.SetCompactionThresholdTokens(config.EffectiveCompactionThresholdTokens(d.Cfg))
	}
	if config.NeedsOnboard(d.Cfg) {
		PrintSystem(d.Out, "Onboard complete (no provider configured; use /onboard to add one)")
	} else {
		PrintSystemf(d.Out, "Onboard complete: %s[%s]", d.Cfg.Current.Model, d.Cfg.Current.Provider)
	}
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	return nil
}

func cloneRootSnapshot(r *config.Root) *config.Root {
	if r == nil {
		return nil
	}
	cp := *r
	if len(r.Providers) > 0 {
		cp.Providers = make(map[string]*config.Provider, len(r.Providers))
		for k, v := range r.Providers {
			if v == nil {
				continue
			}
			p := *v
			cp.Providers[k] = &p
		}
	}
	if len(r.RecentModels) > 0 {
		cp.RecentModels = make(map[string][]string, len(r.RecentModels))
		for k, v := range r.RecentModels {
			cp.RecentModels[k] = append([]string(nil), v...)
		}
	}
	return &cp
}
