//go:build ignore

// Generate the shared browser, callback, and native-app icon assets.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
)

func main() {
	root, err := findRepositoryRoot()
	if err != nil {
		fail(err)
	}
	png, err := logo.IconPNG(1024)
	if err != nil {
		fail(err)
	}
	svg := []byte(logo.IconSVG())
	assets := []struct {
		path string
		data []byte
	}{
		{path: "icon.svg", data: svg},
		{path: "icon.png", data: png},
		{path: "gui/desktop/build/appicon.png", data: png},
		{path: "gui/desktop/assets/icon.png", data: png},
		{path: "gui/desktop/assets/icon.svg", data: svg},
	}
	for _, asset := range assets {
		path := filepath.Join(root, asset.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(path, asset.data, 0o644); err != nil {
			fail(err)
		}
		fmt.Println(filepath.Rel(root, path))
	}
}

func findRepositoryRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(root, "internal", "logo", "logo.txt")); err == nil {
				return root, nil
			}
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", fmt.Errorf("could not locate the Solomon repository root from %s", root)
		}
		root = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
