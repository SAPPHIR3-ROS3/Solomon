package atmention

import (
	"os"
	"path/filepath"
	"strings"
)

func ResolveDocumentPath(tagPath, baseDir string) (abs string, err error) {
	tagPath = strings.TrimSpace(tagPath)
	if tagPath == "" {
		return "", os.ErrNotExist
	}
	baseDir = filepath.Clean(baseDir)
	switch {
	case tagPath == "~":
		return homeDir()
	case strings.HasPrefix(tagPath, "~/"):
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Clean(filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(tagPath, "~/")))), nil
	case strings.HasPrefix(tagPath, "~\\"):
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Clean(filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(tagPath, "~\\")))), nil
	case filepath.IsAbs(tagPath):
		return filepath.Clean(tagPath), nil
	case strings.HasPrefix(tagPath, "/"):
		return filepath.Clean(tagPath), nil
	default:
		if baseDir == "" {
			baseDir, _ = os.Getwd()
		}
		return filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(tagPath))), nil
	}
}

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Clean(home), nil
}

func displayPath(abs, projRoot string) string {
	abs = filepath.Clean(abs)
	if projRoot != "" {
		if rel, err := filepath.Rel(filepath.Clean(projRoot), abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return filepath.ToSlash(rel)
		}
	}
	return abs
}

func underProjectRoot(abs, projRoot string) bool {
	if projRoot == "" {
		return true
	}
	abs = filepath.Clean(abs)
	root := filepath.Clean(projRoot)
	if abs == root {
		return true
	}
	prefix := root + string(os.PathSeparator)
	return strings.HasPrefix(abs, prefix)
}
