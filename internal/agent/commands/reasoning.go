package commands

import (
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func Reasoning(d Deps, parts []string) error {
	if len(parts) < 2 {
		mainLab := d.Cfg.ReasoningEffortLabel()
		subLab := d.Cfg.SubagentReasoningEffortLabel()
		if mainLab == "" {
			mainLab = "unset"
		}
		if subLab == "" {
			subLab = "none"
		}
		PrintSystemf(d.Out, "reasoning_effort=%s (main); subagent_reasoning_effort=%s", mainLab, subLab)
		return nil
	}
	if strings.EqualFold(parts[1], "sub") {
		if len(parts) < 3 {
			if lab := d.Cfg.SubagentReasoningEffortLabel(); lab != "" {
				PrintSystemf(d.Out, "subagent_reasoning_effort=%s", lab)
			} else {
				PrintSystem(d.Out, "subagent_reasoning_effort=none")
			}
			return nil
		}
		canonical, err := config.ParseReasoningEffortToken(parts[2])
		if err != nil {
			return err
		}
		d.Cfg.SubagentReasoningEffort = canonical
		if err := d.SaveCfg(); err != nil {
			return err
		}
		PrintSystemf(d.Out, "subagent_reasoning_effort=%s (saved)", canonical)
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
	PrintSystemf(d.Out, "reasoning_effort=%s (saved; main chat)", canonical)
	return nil
}
