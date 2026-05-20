package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func ConfigBackup(d Deps) error {
	path, err := config.BackupConfig()
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "config backup: %s\n", path)
	return nil
}
