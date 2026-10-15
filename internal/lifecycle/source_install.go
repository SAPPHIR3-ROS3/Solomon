package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

// SourceInstallNeedsStoppedPreparation reports whether dependency repair must wait for runtime shutdown.
// Windows cannot unlink native npm modules loaded by the running Vite server.
func SourceInstallNeedsStoppedPreparation(goos, gui string, state serverruntime.State, ready func() bool) bool {
	if goos != "windows" || state.Mode != "dev" || state.PID <= 0 {
		return false
	}
	devDir, err := filepath.Abs(state.DevDir)
	if err != nil || !strings.EqualFold(filepath.Clean(gui), devDir) {
		return false
	}
	return !ready()
}

// PrepareAndInstallSource runs preparation inside the recoverable runtime handoff only when required.
func PrepareAndInstallSource(stopBeforePrepare bool, prepare, deploy func() error, handoff func(func() error) error) error {
	if !stopBeforePrepare {
		if err := prepare(); err != nil {
			return fmt.Errorf("%w (running service preserved)", err)
		}
	}
	return handoff(func() error {
		if stopBeforePrepare {
			if err := prepare(); err != nil {
				return err
			}
		}
		return deploy()
	})
}
