package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
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
	PrintSystemf(d.Out, "Log level: %s", logging.LevelLabel(lvl))
	return nil
}
