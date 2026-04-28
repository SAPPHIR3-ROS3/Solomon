package commands

import (
	"fmt"

	"solomon/internal/logging"
)

func SlashLog(d Deps, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("usage: /log {error|warning|info|debug|result}")
	}
	lvl, err := logging.ParseLevel(parts[1])
	if err != nil {
		return err
	}
	if err := logging.SetGlobalLevel(lvl); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Log level: %s\n", logging.LevelLabel(lvl))
	return nil
}
