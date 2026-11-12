package dev

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func excluded(name string) bool {
	switch name {
	case ".git", "node_modules", ".dev", "tmp", "temp", "test", "tests", "bin":
		return true
	}
	return strings.HasPrefix(name, ".tmp")
}
func watched(path string, entry fs.DirEntry) bool {
	name := entry.Name()
	if !entry.Type().IsRegular() || strings.HasSuffix(name, "_test.go") || name == "interface.json" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".tmp") {
		return false
	}
	if strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" {
		return true
	}
	if (name == "cursor-dark.json" || name == "display.json") && strings.Contains("/"+filepath.ToSlash(path)+"/", "/syntax/") {
		return true
	}
	if !strings.Contains("/"+filepath.ToSlash(path)+"/", "/fixtures/") {
		return false
	}
	info, err := entry.Info()
	if err != nil || info.Mode()&0111 != 0 {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".exe", ".dll", ".so", ".a", ".o", ".bin":
		return false
	}
	return true
}
func snapshot(roots []string) (map[string][32]byte, error) {
	result := make(map[string][32]byte)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if excluded(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !watched(path, entry) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[path] = sha256.Sum256(data)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func equal(a, b map[string][32]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for p, h := range a {
		if v, ok := b[p]; !ok || v != h {
			return false
		}
	}
	return true
}
