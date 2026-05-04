package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/prompt"
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
	wd := r.ProjRoot
	if p, err := filepath.Abs(r.ProjRoot); err == nil {
		wd = p
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c := newShellCommand(cctx, wd, command)
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

func (r *Runtime) runUserShellLine(ctx context.Context, script string) error {
	wd := r.ProjRoot
	if p, err := filepath.Abs(r.ProjRoot); err == nil {
		wd = p
	}
	c := newShellCommand(ctx, wd, script)
	c.Stdout = r.Out
	c.Stderr = r.Out
	c.Stdin = os.Stdin
	return c.Run()
}

func newShellCommand(ctx context.Context, dir, script string) *exec.Cmd {
	shell := prompt.EffectiveShell()
	if runtime.GOOS == "windows" {
		if shell == "unknown" {
			systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
			if systemRoot == "" {
				systemRoot = strings.TrimSpace(os.Getenv("windir"))
			}
			if systemRoot != "" {
				shell = filepath.Join(systemRoot, "System32", "cmd.exe")
			} else {
				shell = "cmd.exe"
			}
		}
		base := strings.ToLower(filepath.Base(shell))
		var c *exec.Cmd
		switch base {
		case "cmd.exe":
			c = exec.CommandContext(ctx, shell, "/c", script)
		case "powershell.exe", "pwsh.exe":
			c = exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-Command", script)
		default:
			c = exec.CommandContext(ctx, shell, "-c", script)
		}
		c.Dir = dir
		return c
	}
	if shell == "unknown" {
		c := exec.CommandContext(ctx, "sh", "-c", script)
		c.Dir = dir
		return c
	}
	c := exec.CommandContext(ctx, shell, "-c", script)
	c.Dir = dir
	return c
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
		r.checkpointStageProjAbs(p)
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
	r.checkpointStageProjAbs(p)
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
