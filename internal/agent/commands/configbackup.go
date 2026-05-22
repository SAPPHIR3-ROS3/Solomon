package commands

import (
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func ConfigBackup(d Deps) error {
	path, err := config.BackupConfig()
	if err != nil {
		return err
	}
	PrintSystemf(d.Out, "config backup: %s", path)
	return nil
}
