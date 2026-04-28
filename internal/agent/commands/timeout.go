package commands

import (
	"fmt"
	"strconv"

	"solomon/internal/config"
)

func Timeout(d Deps, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("usage: /timeout <minutes>")
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	if err := config.ClampTimeoutMinutes(n); err != nil {
		return err
	}
	d.Cfg.SubagentTimeoutMinutes = n
	if err := d.SaveCfg(); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "subagent_timeout_minutes=%d\n", n)
	return nil
}
