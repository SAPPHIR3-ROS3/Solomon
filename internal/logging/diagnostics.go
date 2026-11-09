package logging

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var diagnosticSecret = regexp.MustCompile(`(?i)\b(password|passwd|token|api[_-]?key|authorization|secret|client[_-]?secret|access[_-]?token|refresh[_-]?token)\b(\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,;]+)`)

var processMemory = func(map[string]any) {}

type requestKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestKey{}).(string)
	return id
}

func Diagnostic(event string, params map[string]any) {
	logMu.Lock()
	ready := globalLogLevelInitialized
	logMu.Unlock()
	if !ready {
		return
	}
	clean := make(map[string]any, len(params)+2)
	clean["source_pid"] = os.Getpid()
	for key, value := range params {
		if text, ok := value.(string); ok {
			text = strings.ReplaceAll(strings.ReplaceAll(text, "\n", "\\n"), "\r", "\\r")
			if len(text) > 2048 {
				text = text[:2048] + "..."
			}
			value = diagnosticSecret.ReplaceAllString(RedactReport(text), "${1}${2}<secret>")
		}
		clean[key] = value
	}
	level := INFO_LOG_LEVEL
	for _, key := range []string{"error", "error_type", "error_name", "write_error", "panic_type"} {
		if _, ok := clean[key]; ok {
			level = WARNING_LOG_LEVEL
		}
	}
	if status, ok := clean["status"].(int); ok && status >= 400 {
		level = WARNING_LOG_LEVEL
	}
	if status, ok := clean["status"].(int); ok && status >= 500 {
		level = ERROR_LOG_LEVEL
	}
	if _, ok := clean["panic_type"]; ok {
		level = ERROR_LOG_LEVEL
	}
	if clean["outcome"] == "canceled" {
		level = INFO_LOG_LEVEL
	}
	clean["severity"] = LevelLabel(level)
	console := false
	Log(level, event, LogOptions{Params: clean, WriteConsole: &console, Diagnostic: true, CallerSkip: 1})
}

func ProcessFinished(ctx context.Context, cmd *exec.Cmd, operation string, started time.Time, err error) {
	if err == nil && ctx.Err() == nil && time.Since(started) < time.Second {
		return
	}
	params := map[string]any{"operation": operation, "duration_ms": time.Since(started).Milliseconds(), "outcome": "success"}
	if id := RequestID(ctx); id != "" {
		params["request_id"] = id
	}
	if cmd != nil {
		params["executable"] = filepath.Base(cmd.Path)
		if cmd.Process != nil {
			params["pid"] = cmd.Process.Pid
		}
		if cmd.ProcessState != nil {
			params["exit_code"] = cmd.ProcessState.ExitCode()
			params["process_state"] = cmd.ProcessState.String()
		}
	}
	if err != nil {
		params["outcome"] = "failed"
		params["error_type"] = fmt.Sprintf("%T", err)
		params["error"] = err.Error()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() < 0 {
				processMemory(params)
			}
			params["process_state"] = exit.ProcessState.String()
		}
	}
	if ctx.Err() != nil {
		params["context_error"] = ctx.Err().Error()
		params["outcome"] = "canceled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			params["outcome"] = "timeout"
		}
	}
	Diagnostic("process finished", params)
}

func ProcessTermination(pid int, operation, reason string) {
	if pid > 1 {
		Diagnostic("process termination requested", map[string]any{"pid": pid, "operation": operation, "reason": reason})
	}
}
