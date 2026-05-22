package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
)

func BackupConfig() (string, error) {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "config backup path resolve failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	src, err := os.Open(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Log(logging.ERROR_LOG_LEVEL, "config backup source not found", logging.LogOptions{Params: map[string]any{"path": cfgPath}})
			return "", fmt.Errorf("config not found at %s", cfgPath)
		}
		logging.Log(logging.ERROR_LOG_LEVEL, "config backup open source failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})
		return "", err
	}
	defer src.Close()
	home, err := paths.SolomonHome()
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "config backup solomon home failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	backupDir := filepath.Join(home, "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "config backup mkdir failed", logging.LogOptions{Params: map[string]any{"path": backupDir, "err": err.Error()}})
		return "", err
	}
	iso := time.Now().Format("2006-01-02T15-04-05.000Z07-00")
	dstPath := filepath.Join(backupDir, "config.toml."+iso+".bak")
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "config backup create dest failed", logging.LogOptions{Params: map[string]any{"path": dstPath, "err": err.Error()}})
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(dstPath)
		logging.Log(logging.ERROR_LOG_LEVEL, "config backup copy failed", logging.LogOptions{Params: map[string]any{"path": dstPath, "err": err.Error()}})
		return "", err
	}
	logging.Log(logging.INFO_LOG_LEVEL, "config backup created", logging.LogOptions{Params: map[string]any{"path": dstPath}})
	return dstPath, nil
}
