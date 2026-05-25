package cievents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
)

const (
	ExitOK      = 0
	ExitGeneric = 1
	ExitUsage   = 2
	ExitConfig  = 3
	ExitLLM     = 4
	ExitTool    = 5
	ExitTimeout = 6
)

type RunError struct {
	Code   int
	Reason string
	Err    error
}

func (e *RunError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Reason
}

func (e *RunError) Unwrap() error { return e.Err }

func NewRunError(code int, reason string, err error) *RunError {
	return &RunError{Code: code, Reason: reason, Err: err}
}

func ClassifyExit(err error) (code int, reason string) {
	if err == nil {
		return ExitOK, "ok"
	}
	var re *RunError
	if errors.As(err, &re) {
		r := re.Reason
		if r == "" {
			r = "error"
		}
		return re.Code, r
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ExitTimeout, "timeout"
	}
	if errors.Is(err, llm.ErrStreamAccumulatorRejected) {
		return ExitLLM, "stream_integrity"
	}
	msg := err.Error()
	for _, sub := range []string{"rate limit", "429", "500", "502", "503", "API", "stream"} {
		if strings.Contains(msg, sub) {
			return ExitLLM, "api_error"
		}
	}
	return ExitGeneric, "error"
}

func ConfigError(err error) error {
	return NewRunError(ExitConfig, "config", err)
}

func UsageError(msg string) error {
	return NewRunError(ExitUsage, "usage", fmt.Errorf("%s", msg))
}

func ToolPolicyError() error {
	return NewRunError(ExitTool, "tool_error", errors.New("tool returned error"))
}

func TimeoutError(err error) error {
	return NewRunError(ExitTimeout, "timeout", err)
}

type ExecCLIOpts struct {
	JSON            bool
	JSONL           bool
	NoColor         bool
	FailOnToolError bool
	EnvFile         string
	Prompt          string
}

func ParseExecCLIArgs(args []string) (ExecCLIOpts, error) {
	var o ExecCLIOpts
	posStart := -1
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			o.JSON = true
		case "--jsonl":
			o.JSONL = true
		case "--no-color":
			o.NoColor = true
		case "--fail-on-tool-error":
			o.FailOnToolError = true
		case "--env-file":
			if i+1 >= len(args) {
				return o, UsageError("missing value for --env-file")
			}
			i++
			o.EnvFile = args[i]
		default:
			if strings.HasPrefix(a, "-") {
				return o, UsageError("unknown flag: " + a)
			}
			if posStart < 0 {
				posStart = i
			}
		}
	}
	if posStart < 0 {
		return o, nil
	}
	for _, t := range args[posStart:] {
		if strings.HasPrefix(t, "-") {
			return o, UsageError("prompt must be last (no flags after positional text)")
		}
	}
	o.Prompt = strings.TrimSpace(strings.Join(args[posStart:], " "))
	return o, nil
}
