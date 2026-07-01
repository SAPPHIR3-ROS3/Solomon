package agentruntime

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"

func UsesExecInstallRestartAfterSystemInstallForTest() bool {
	return updater.UsesExecInstallRestartAfterSystemInstall()
}
