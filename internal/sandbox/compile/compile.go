package compile

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ModuleDir() (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/SAPPHIR3-ROS3/Solomon/v2026").Output()
	if err != nil {
		return "", fmt.Errorf("resolve module dir: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

type Options struct {
	Source     string
	ModuleRoot string
	CacheDir   string
}

func BuildWASM(opts Options) ([]byte, error) {
	if strings.TrimSpace(opts.Source) == "" {
		return nil, fmt.Errorf("orchestrate: empty source")
	}
	modRoot := opts.ModuleRoot
	if modRoot == "" {
		var err error
		modRoot, err = ModuleDir()
		if err != nil {
			return nil, err
		}
	}
	cacheBase := filepath.Join(modRoot, ".solomon")
	if err := os.MkdirAll(cacheBase, 0o755); err != nil {
		return nil, err
	}
	slotDir, err := os.MkdirTemp(cacheBase, "orchestrate-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(slotDir)

	src := opts.Source
	if !strings.Contains(src, "package main") {
		return nil, fmt.Errorf("orchestrate: source must contain package main")
	}
	if err := os.WriteFile(filepath.Join(slotDir, "main.go"), []byte(src), 0o600); err != nil {
		return nil, err
	}
	outPath := filepath.Join(slotDir, "script.wasm")
	relSlot, err := filepath.Rel(modRoot, slotDir)
	if err != nil {
		return nil, err
	}
	pkg := "./" + filepath.ToSlash(relSlot)
	cmd := exec.Command("go", "build", "-o", outPath, pkg)
	cmd.Dir = modRoot
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	if opts.CacheDir != "" {
		cmd.Env = append(cmd.Env, "GOCACHE="+opts.CacheDir)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("compile: %s", msg)
	}
	return os.ReadFile(outPath)
}

func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(home, ".solomon", "cache", "go-build")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}
