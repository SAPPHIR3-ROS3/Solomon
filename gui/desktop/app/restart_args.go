package app

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
	"os"
)

func desktopRestartArgs() []string {
	return updater.DesktopRestartArgs(os.Args[0])
}
