// Package modelcatalogcache persists the provider model list for one local day.
package modelcatalogcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const filename = "model-catalog-cache.json"

type document struct {
	Day       string          `json:"day"`
	Providers json.RawMessage `json:"providers"`
}

// LoadToday restores a provider list only when it was saved today in the
// user's local timezone. The config TOML remains the source of current and
// recent selections.
func LoadToday(providers any) (bool, error) {
	path, err := cachePath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var saved document
	if err := json.Unmarshal(data, &saved); err != nil {
		return false, err
	}
	if saved.Day != today() || len(saved.Providers) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(saved.Providers, providers); err != nil {
		return false, err
	}
	return true, nil
}

func SaveToday(providers any) error {
	encoded, err := json.Marshal(providers)
	if err != nil {
		return err
	}
	doc, err := json.Marshal(document{Day: today(), Providers: encoded})
	if err != nil {
		return err
	}
	path, err := cachePath()
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, doc, 0o600); err != nil {
		return fmt.Errorf("write model catalog cache: %w", err)
	}
	return os.Rename(temporary, path)
}

func cachePath() (string, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, filename), nil
}

func today() string {
	return time.Now().Format("2006-01-02")
}
