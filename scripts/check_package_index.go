//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "check_package_index: %v\n", err)
		os.Exit(2)
	}

	indexPath := filepath.Join(root, "docs/architecture/package-index.md")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check_package_index: read index: %v\n", err)
		os.Exit(2)
	}
	indexText := string(indexData)

	var pkgs []string
	for _, scanRoot := range []string{"internal", "cmd"} {
		walkRoot := filepath.Join(root, scanRoot)
		_ = filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			if skipDir(path) {
				return filepath.SkipDir
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil
			}
			hasGo := false
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
					hasGo = true
					break
				}
			}
			if !hasGo {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			pkgs = append(pkgs, filepath.ToSlash(rel))
			return nil
		})
	}

	var missing []string
	for _, pkg := range pkgs {
		slash := pkg + "/"
		backtick := "`" + slash + "`"
		if !strings.Contains(indexText, slash) && !strings.Contains(indexText, backtick) {
			missing = append(missing, pkg)
		}
	}

	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "check_package_index: %d package(s) missing from docs/architecture/package-index.md:\n", len(missing))
		for _, p := range missing {
			fmt.Fprintf(os.Stderr, "  %s/\n", p)
		}
		os.Exit(1)
	}
	fmt.Println("check_package_index: ok")
}

func skipDir(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}
	switch base {
	case "bundle", "node_modules", "dist":
		return true
	}
	return false
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
