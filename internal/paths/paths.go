package paths

import (
	"os"
	"path/filepath"
)

func SolomonHome() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".solomon"), nil
}

func ConfigPath() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.toml"), nil
}

func ProjectsMapPath() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "projectsId.json"), nil
}

func ProjectsDir() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "projects"), nil
}

func ProjectRoot(projectHexID string) (string, error) {
	dir, err := ProjectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, projectHexID), nil
}
