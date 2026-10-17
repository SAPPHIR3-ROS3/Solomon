package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	toml "github.com/pelletier/go-toml/v2"
)

var ErrInvalidServerDevelopment = errors.New("invalid dev server settings")

type ServerSettings struct {
	DevMode      *bool  `toml:"dev_mode,omitempty"`
	DevDirectory string `toml:"dev_directory,omitempty"`
}

// NormalizeDevDirectory expands a source path without requiring existing files.
func NormalizeDevDirectory(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	// Windows' Copy as path includes surrounding quotes.
	if len(raw) >= 2 && ((raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')) {
		raw = raw[1 : len(raw)-1]
	}
	if raw == "" {
		return "", fmt.Errorf("set a GUI source directory before enabling the dev server")
	}
	if raw == "~" || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if raw == "~" {
			raw = home
		} else {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "~/"))
		}
	}
	directory, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	return directory, nil
}

func ValidateDevDirectory(raw string) (string, error) {
	directory, err := NormalizeDevDirectory(raw)
	if err != nil {
		return "", err
	}
	// Accept the repository folder as well as its GUI subdirectory.
	if info, err := os.Stat(filepath.Join(directory, "gui", "package.json")); err == nil && !info.IsDir() {
		if info, err := os.Stat(filepath.Join(directory, "gui", "src")); err == nil && info.IsDir() {
			directory = filepath.Join(directory, "gui")
		}
	}
	for _, required := range []string{"package.json", "src"} {
		info, err := os.Stat(filepath.Join(directory, required))
		if err != nil || (required == "src" && !info.IsDir()) || (required == "package.json" && info.IsDir()) {
			return "", fmt.Errorf("development GUI directory is invalid: missing %s in %s", required, directory)
		}
	}
	return directory, nil
}

// ResolveServerDevelopment applies saved preferences to the default mode.
// Explicit CLI dev/normal arguments bypass this function.
func ResolveServerDevelopment(mode, directory string) (string, string, error) {
	path, err := paths.ConfigPath()
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return mode, directory, nil
	}
	if err != nil {
		return "", "", err
	}
	var file struct {
		Server ServerSettings `toml:"server"`
	}
	if err := toml.Unmarshal(data, &file); err != nil {
		return "", "", err
	}
	if file.Server.DevMode == nil {
		return mode, directory, nil
	}
	if !*file.Server.DevMode {
		return "normal", "", nil
	}
	directory, err = ValidateDevDirectory(file.Server.DevDirectory)
	if err != nil {
		return "", "", err
	}
	return "dev", directory, nil
}
