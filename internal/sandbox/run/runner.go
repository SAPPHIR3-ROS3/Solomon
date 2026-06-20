package run

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/host"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Config struct {
	MaxToolCalls int
	Timeout      time.Duration
}

type Runner struct {
	mu      sync.Mutex
	runtime wazero.Runtime
	cache   map[string]wazero.CompiledModule
	caller  host.ToolCaller
	cfg     Config
}

func NewRunner(ctx context.Context, caller host.ToolCaller, cfg Config) (*Runner, error) {
	r := wazero.NewRuntime(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		r.Close(ctx)
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox runner wasi instantiate failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	if err := host.Register(ctx, r, caller); err != nil {
		r.Close(ctx)
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox runner host register failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	if cfg.MaxToolCalls <= 0 {
		cfg.MaxToolCalls = 256
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	return &Runner{runtime: r, cache: make(map[string]wazero.CompiledModule), caller: caller, cfg: cfg}, nil
}

func (rn *Runner) Close(ctx context.Context) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if rn.runtime != nil {
		rn.runtime.Close(ctx)
		rn.runtime = nil
	}
	rn.cache = nil
}

func (rn *Runner) Run(ctx context.Context, wasm []byte) (output string, err error) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if rn.runtime == nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "sandbox run on closed runner")
		return "", fmt.Errorf("runner closed")
	}
	h := sha256.Sum256(wasm)
	key := hex.EncodeToString(h[:])
	compiled, ok := rn.cache[key]
	if !ok {
		var errCompile error
		compiled, errCompile = rn.runtime.CompileModule(ctx, wasm)
		if errCompile != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "sandbox wasm compile failed", logging.LogOptions{Params: map[string]any{"err": errCompile.Error()}})
			return "", errCompile
		}
		rn.cache[key] = compiled
	}
	var stdout, stderr bytes.Buffer
	cfg := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithName("solomon-script-" + key[:8])
	runCtx, cancel := context.WithTimeout(ctx, rn.cfg.Timeout)
	defer cancel()
	mod, err := rn.runtime.InstantiateModule(runCtx, compiled, cfg)
	if err != nil {
		params := map[string]any{"err": err.Error()}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			logging.Log(logging.ERROR_LOG_LEVEL, "sandbox module instantiate failed", logging.LogOptions{Params: map[string]any{"err": err.Error(), "stderr": msg}})
			return stdout.String(), fmt.Errorf("%v: %s", err, msg)
		}
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox module instantiate failed", logging.LogOptions{Params: params})
		return stdout.String(), err
	}
	defer mod.Close(runCtx)
	if cc, ok := rn.caller.(*host.CountingCaller); ok && cc.LastError != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "sandbox tool call failed", logging.LogOptions{Params: map[string]any{"err": cc.LastError.Error()}})
		return stdout.String(), cc.LastError
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" && stderr.Len() > 0 {
		out = strings.TrimSpace(stderr.String())
	}
	return out, nil
}
