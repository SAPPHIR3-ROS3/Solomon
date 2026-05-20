package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
)

func BackupConfig() (string, error) {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return "", err
	}
	src, err := os.Open(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("config not found at %s", cfgPath)
		}
		return "", err
	}
	defer src.Close()
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	backupDir := filepath.Join(home, "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", err
	}
	iso := time.Now().Format("2006-01-02T15-04-05.000Z07-00")
	dstPath := filepath.Join(backupDir, "config.toml."+iso+".bak")
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(dstPath)
		return "", err
	}
	return dstPath, nil
}
