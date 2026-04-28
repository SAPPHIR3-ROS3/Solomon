package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type shellArgs struct {
	Command string `json:"command"`
}

func (r *Runtime) toolShell(ctx context.Context, raw json.RawMessage) (any, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, err
	}
	cmdRaw, ok := rawMap["command"]
	if !ok {
		return nil, fmt.Errorf("command required")
	}
	var command string
	if err := json.Unmarshal(cmdRaw, &command); err != nil {
		return nil, err
	}
	sec := parseOptionalTimeoutSecs(rawMap)
	timeout := time.Minute
	if sec != nil {
		timeout = time.Duration(*sec) * time.Second
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c := exec.CommandContext(cctx, "sh", "-c", command)
	c.Dir = wd
	out, err := c.CombinedOutput()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		}
	}
	m := map[string]any{"exit": exit, "output": string(out)}
	if err != nil && !errors.As(err, new(*exec.ExitError)) {
		m["shell_error"] = err.Error()
	}
	return m, nil
}

func parseOptionalTimeoutSecs(m map[string]json.RawMessage) *int {
	v, ok := m["timeoutSeconds"]
	if !ok {
		return nil
	}
	var n int
	if json.Unmarshal(v, &n) != nil {
		return nil
	}
	return &n
}

type readArgs struct {
	Path string `json:"path"`
}

func (r *Runtime) toolReadFile(raw json.RawMessage) (any, error) {
	var a readArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p := resolveProjectPath(r.ProjRoot, a.Path)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": p, "content": string(b)}, nil
}

type editArgs struct {
	Path      string `json:"path"`
	OldString string `json:"oldString"`
	NewString string `json:"newString"`
}

func (r *Runtime) toolEditFile(raw json.RawMessage) (any, error) {
	var a editArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p := resolveProjectPath(r.ProjRoot, a.Path)
	if a.OldString == "" {
		if err := os.WriteFile(p, []byte(a.NewString), 0o600); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "action": "created_or_overwrite"}, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	s := string(b)
	if !strings.Contains(s, a.OldString) {
		return map[string]any{"ok": false, "reason": "oldString not found"}, nil
	}
	s = strings.Replace(s, a.OldString, a.NewString, 1)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "action": "edited"}, nil
}

type subagentArgs struct {
	SysPromptPath string `json:"sysPromptPath"`
	Task          string `json:"task"`
}

func (r *Runtime) toolSubagent(ctx context.Context, raw json.RawMessage) (any, error) {
	var a subagentArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p := resolveProjectPath(r.ProjRoot, a.SysPromptPath)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	sys := string(b)
	prev := r.Mode
	r.Mode = "build"
	out, err := r.runNestedWithSystem(ctx, sys, a.Task)
	r.Mode = prev
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "output": out}, nil
}

func resolveProjectPath(root, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, filepath.Clean(p))
}
