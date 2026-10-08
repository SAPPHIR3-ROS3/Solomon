package compile

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type Options struct {
	Source     string
	ModuleRoot string
	CacheDir   string
	MCPTools   []MCPToolBinding
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
	modRoot, err := filepath.Abs(modRoot)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cacheBase := filepath.Join(home, ".solomon", "cache", "orchestrate")
	if err := os.MkdirAll(cacheBase, 0o755); err != nil {
		return nil, err
	}
	slotDir, err := os.MkdirTemp(cacheBase, "orchestrate-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(slotDir)

	src, parseErr := RewriteSDKImports(opts.Source)
	if parseErr != nil {
		return nil, fmt.Errorf("compile: %s", orchestrateParseError(parseErr, opts.Source))
	}
	src, parseErr = RewriteMCPCalls(src, opts.MCPTools)
	if parseErr != nil {
		return nil, fmt.Errorf("compile: %s", orchestrateParseError(parseErr, opts.Source))
	}
	if !strings.Contains(src, "package main") {
		return nil, fmt.Errorf("orchestrate: source must contain package main")
	}
	if err := os.WriteFile(filepath.Join(slotDir, "main.go"), []byte(src), 0o600); err != nil {
		return nil, err
	}
	outPath := filepath.Join(slotDir, "script.wasm")
	// A child module path permits internal SDK imports without placing files or
	// overlays inside Solomon's module tree (Go rejects overlays in GOMODCACHE).
	goMod := fmt.Sprintf("module %s/orchestrate\n\ngo 1.25.0\n\nrequire %s v2026.0.0\n\nreplace %s => %q\n", SolomonModulePath, SolomonModulePath, SolomonModulePath, modRoot)
	if err := os.WriteFile(filepath.Join(slotDir, "go.mod"), []byte(goMod), 0o600); err != nil {
		return nil, err
	}
	if sums, err := os.ReadFile(filepath.Join(modRoot, "go.sum")); err == nil {
		if err := os.WriteFile(filepath.Join(slotDir, "go.sum"), sums, 0o600); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	cmd := exec.Command("go", "build", "-mod=mod", "-o", outPath, ".")
	cmd.Dir = slotDir
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0", "GOWORK=off")
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
		compileErr := fmt.Errorf("compile: %s", clarifyCompileError(opts.Source, msg))
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox orchestrate compile failed", logging.LogOptions{Params: map[string]any{"err": compileErr.Error()}})
		return nil, compileErr
	}
	wasm, err := os.ReadFile(outPath)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox wasm read failed", logging.LogOptions{Params: map[string]any{"path": outPath, "err": err.Error()}})
		return nil, err
	}
	return wasm, nil
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
