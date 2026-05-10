package tools

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
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureShell(command, intent string) {}

func shellOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("shell", "Run a shell command in the harness working directory.", map[string]any{
		"command": map[string]any{"type": "string", "description": "Shell command to run"},
		"intent":  map[string]any{"type": "string", "description": "Brief phrase describing why this command is being run"},
		"timeoutSeconds": map[string]any{
			"type":        "integer",
			"description": "Optional timeout in seconds for this command",
		},
	}, []string{"command", "intent"})
}

func appendShellDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureShell)
	if err != nil {
		return err
	}
	b.addBlock("shell", "Run a shell command in the harness working directory. Requires intent (brief purpose). Optional JSON fields may tweak behavior.", sig)
	return nil
}

func execShell(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
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
	intentRaw, ok := rawMap["intent"]
	if !ok {
		return nil, fmt.Errorf("intent required")
	}
	var intent string
	if err := json.Unmarshal(intentRaw, &intent); err != nil {
		return nil, err
	}
	if strings.TrimSpace(intent) == "" {
		return nil, fmt.Errorf("intent must be a non-empty brief phrase")
	}
	sec := parseOptionalTimeoutSecs(rawMap)
	timeout := time.Minute
	if sec != nil {
		timeout = time.Duration(*sec) * time.Second
	}
	wd := env.ProjRoot
	if p, err := filepath.Abs(env.ProjRoot); err == nil {
		wd = p
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c := NewShellCommand(cctx, wd, command)
	out, err := c.CombinedOutput()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		}
	}
	m := map[string]any{"exit": exit, "output": string(out), "intent": intent}
	if err != nil && !errors.As(err, new(*exec.ExitError)) {
		m["shell_error"] = err.Error()
	}
	return m, nil
}

func NewShellCommand(ctx context.Context, dir, script string) *exec.Cmd {
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
