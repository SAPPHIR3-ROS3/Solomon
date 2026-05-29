package connect

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/providersetup"
)

func Run(d Deps) error {
	kind, skipped, err := config.ChooseProviderKind(PromptIO(d), false, "Connect provider type:")
	if err != nil {
		return err
	}
	if skipped {
		return fmt.Errorf("missing selection")
	}
	return runKind(d, kind)
}

func runKind(d Deps, kind int) error {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	setupRes, err := providersetup.RunProviderSetupByKind(ctx, PromptIO(d), d.Cfg, d.Cfg, kind, ProviderSetupOpts(d))
	if err != nil {
		return err
	}
	if setupRes == nil {
		return nil
	}
	if setupRes.SwitchCurrent && d.ApplyCurrentModel != nil {
		if err := d.ApplyCurrentModel(setupRes.CurrentProvider, setupRes.CurrentModel); err != nil {
			return err
		}
		printSystemf(d, "Using %s[%s]", d.Model(), d.Provider().Name)
	}
	return nil
}

func ProviderSetupOpts(d Deps) config.ProviderSetupOpts {
	return config.ProviderSetupOpts{
		RequireProvider: true,
		WriteToConfig:   true,
		SaveConfig:      d.SaveCfg,
	}
}
