package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type Export struct {
	Path string `toml:"path,omitempty"`
}

func (r *Root) EffectiveExportRoot() (string, error) {
	if r != nil {
		if p := strings.TrimSpace(r.Export.Path); p != "" {
			return expandExportPath(p)
		}
	}
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "exported"), nil
}

func expandExportPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	if strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, raw[2:]), nil
	}
	return raw, nil
}
