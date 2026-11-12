package tools

import (
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/validation"
	"strings"
	"sync"
)

const maxOrchestrateToolErrors = 16

const outcomeGuidance = "Inspect the orchestration result status and failed_tool_calls. status=success means the script finished and no host tool reported failure; status=partial means the script finished but one or more host calls failed, even if their errors were caught or printed. script_ok describes script execution only. Check tool_errors and report any handled fallback or remaining failure; do not claim clean success from script_ok alone. Failed host calls do not stop later calls or trigger an automatic replay."

type orchestrateToolError struct {
	Call     int    `json:"call"`
	Tool     string `json:"tool"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

type orchestrateOutcomes struct {
	mu     sync.Mutex
	calls  int
	failed int
	errors []orchestrateToolError
}

func (o *orchestrateOutcomes) record(name string, raw json.RawMessage, callErr error) {
	kind, message := toolFailure(name, raw, callErr)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls++
	if kind == "" {
		return
	}
	o.failed++
	if len(o.errors) < maxOrchestrateToolErrors {
		runes := []rune(message)
		if len(runes) > 400 {
			message = string(runes[:400]) + "…"
		}
		category := "tool"
		if callErr != nil {
			category = compile.DescribeFailure(callErr).Kind
			if issue := validation.Kind(callErr); issue != "" {
				category = issue
			}
			if category == "environment" {
				category = "tool"
			}
		} else if name == "shell" {
			category = "command"
		}
		o.errors = append(o.errors, orchestrateToolError{Call: o.calls, Tool: name, Kind: kind, Category: category, Message: message})
	}
}

func (o *orchestrateOutcomes) apply(out map[string]any, scriptOK bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	status := "success"
	if !scriptOK {
		status = "error"
	} else if o.failed > 0 {
		status = "partial"
	}
	out["ok"] = scriptOK && o.failed == 0
	out["script_ok"] = scriptOK
	out["status"] = status
	out["observed_tool_calls"] = o.calls
	out["failed_tool_calls"] = o.failed
	out["successful_tool_calls"] = o.calls - o.failed
	if o.failed > 0 {
		out["tool_errors"] = append([]orchestrateToolError(nil), o.errors...)
		if o.failed > len(o.errors) {
			out["tool_errors_truncated"] = true
		}
	}
}

func toolFailure(name string, raw json.RawMessage, callErr error) (string, string) {
	if callErr != nil {
		return "host_error", callErr.Error()
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(raw, &result) != nil || result == nil {
		return "", ""
	}
	if name == "shell" {
		if message := resultErrorText(result["shell_error"]); message != "" {
			return "shell_error", message
		}
		var exit int
		if json.Unmarshal(result["exit"], &exit) == nil && exit != 0 {
			return "shell_exit", fmt.Sprintf("shell exited with code %d", exit)
		}
	}
	if strings.TrimSpace(string(result["ok"])) == "false" {
		message := resultErrorText(result["reason"])
		if message == "" {
			message = resultErrorText(result["error"])
		}
		if message == "" {
			message = "tool returned ok=false"
		}
		return "tool_error", message
	}
	var isError bool
	if json.Unmarshal(result["isError"], &isError) == nil && isError {
		return "tool_error", "tool returned isError=true"
	}
	if message := resultErrorText(result["error"]); message != "" {
		return "tool_error", message
	}
	return "", ""
}

func resultErrorText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "false" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	return string(raw)
}
