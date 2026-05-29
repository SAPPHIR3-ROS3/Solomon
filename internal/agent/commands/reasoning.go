package commands

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func Reasoning(d Deps, parts []string) error {
	if len(parts) < 2 {
		if lab := d.Cfg.ReasoningEffortLabel(); lab != "" {
			PrintSystemf(d.Out, "reasoning_effort=%s (main chat); subagent always none", lab)
		} else {
			PrintSystem(d.Out, "reasoning_effort unset for main chat (provider default); subagent always none")
		}
		return nil
	}
	canonical, err := config.ParseReasoningEffortToken(parts[1])
	if err != nil {
		return err
	}
	d.Cfg.ReasoningEffort = canonical
	if err := d.SaveCfg(); err != nil {
		return err
	}
	PrintSystemf(d.Out, "reasoning_effort=%s (saved; main chat); subagent always none", canonical)
	return nil
}
