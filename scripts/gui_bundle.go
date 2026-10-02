//go:build ignore

// gui_bundle stages Vite output for embedding into both Solomon binaries.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	if err := bundle(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bundle() error {
	source := filepath.Join("gui", "dist")
	target := filepath.Join("gui", "assets", "frontend")
	if _, err := os.Stat(filepath.Join(source, "index.html")); err != nil {
		return err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != ".keep" {
			if err := os.RemoveAll(filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, contents, 0o644)
	})
}
