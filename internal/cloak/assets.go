package cloak

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed *.mjs diagnostics
var assets embed.FS

func installAssets(directory string) error {
	return fs.WalkDir(assets, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || path == "bridge.mjs" {
			return nil
		}
		content, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(directory, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, content) {
			return nil
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		return nil
	})
}
