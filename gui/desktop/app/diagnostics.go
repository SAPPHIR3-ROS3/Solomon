package app

import (
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func configureDiagnostics() error {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	home, err := paths.SolomonHome()
	if err != nil {
		return err
	}
	if err := logging.Configure(logging.Config{Dir: filepath.Join(home, "logs"), WriteFile: true, Retention: 7}); err != nil {
		return err
	}
	if cfg, err := config.LoadOptional(); err == nil && cfg != nil && cfg.LogLevel != "" {
		if level, err := logging.ParseLevel(cfg.LogLevel); err == nil {
			_ = logging.SetGlobalLevel(level)
		}
	}
	logging.Diagnostic("desktop starting", nil)
	return nil
}
