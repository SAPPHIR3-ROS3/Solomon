package commands

import (
	"fmt"

	"solomon/internal/config"
)

func Reasoning(d Deps, parts []string) error {
	if len(parts) < 2 {
		if lab := d.Cfg.ReasoningEffortLabel(); lab != "" {
			fmt.Fprintf(d.Out, "reasoning_effort=%s (main chat only; subagent omits reasoning)\n", lab)
		} else {
			fmt.Fprintln(d.Out, "reasoning_effort unset (provider default); subagent never sends reasoning_effort")
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
	fmt.Fprintf(d.Out, "reasoning_effort=%s (saved; main chat only)\n", canonical)
	return nil
}
