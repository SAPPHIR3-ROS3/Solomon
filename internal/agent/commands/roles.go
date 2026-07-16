package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func AddSubagent(d Deps) error {
	if d.Cfg == nil || d.SaveCfg == nil {
		return fmt.Errorf("/add subagent unavailable")
	}
	pio := PromptIO(d)
	if err := config.EnsureRolesTable(pio, d.Cfg); err != nil {
		return err
	}
	if d.SaveCfg != nil {
		if err := d.SaveCfg(); err != nil {
			return err
		}
	}
	lm, err := PickListedModel(d, true)
	if err != nil {
		return err
	}
	res, err := config.CompleteSubagentEntry(d.Ctx, pio, d.Cfg, lm.Prov, lm.Model)
	if err != nil {
		return err
	}
	previousCount := len(d.Cfg.Roles.Subagent)
	config.ApplySubagentAdd(d.Cfg, res)
	if err := d.SaveCfg(); err != nil {
		d.Cfg.Roles.Subagent = d.Cfg.Roles.Subagent[:previousCount]
		return err
	}
	for _, w := range config.RolesScoreWarnings(d.Cfg) {
		PrintSystem(d.Out, "warning: "+w)
	}
	msg := fmt.Sprintf("Subagent added: %s[%s] (scores assigned manually)", res.Model, res.Provider)
	PrintSystem(d.Out, msg)
	return nil
}
