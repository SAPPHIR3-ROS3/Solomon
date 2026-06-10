package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ReferenceWASMPath(version string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	dir := filepath.Join(home, ".solomon", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "wasm-sdk-"+v+".wasm"), nil
}

func EnsureReferenceWASM(version string) ([]byte, error) {
	path, err := ReferenceWASMPath(version)
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return b, nil
	}
	modRoot, err := ModuleDir()
	if err != nil {
		return nil, err
	}
	cacheDir, _ := CacheDir()
	b, err := BuildWASM(Options{
		Source:     "package main\n\nfunc main() {}\n",
		ModuleRoot: modRoot,
		CacheDir:   cacheDir,
	})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return nil, fmt.Errorf("write reference wasm: %w", err)
	}
	return b, nil
}

func WarmSDKCache() error {
	_, err := EnsureReferenceWASM("")
	return err
}
