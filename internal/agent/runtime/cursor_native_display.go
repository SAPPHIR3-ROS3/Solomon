package agentruntime

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

type cursorNativeToolEvent struct {
	CallID string          `json:"callId"`
	Name   string          `json:"name"`
	Status string          `json:"status"`
	Args   json.RawMessage `json:"args"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

func (r *Runtime) PrintCursorNativeToolEvent(rawJSON string) {
	r.printCursorNativeToolEvent(rawJSON)
}

func (r *Runtime) CursorNativeToolsEnabled() bool {
	return r.cursorNativeToolsEnabled()
}

func (r *Runtime) cursorNativeToolsEnabled() bool {
	return r != nil && r.Prov != nil && r.Prov.IsCursorAPI() &&
		r.Cfg != nil && r.Cfg.Tools.CursorInternalTools
}

func (r *Runtime) printCursorNativeToolEvent(rawJSON string) {
	if r == nil || r.Out == nil || strings.TrimSpace(rawJSON) == "" {
		return
	}
	var ev cursorNativeToolEvent
	if err := json.Unmarshal([]byte(rawJSON), &ev); err != nil {
		return
	}
	name := strings.TrimSpace(ev.Name)
	if name == "" {
		name = "tool"
	}
	label := name + " (cursor)"
	switch strings.TrimSpace(ev.Status) {
	case "running":
		body := cursorNativeArgsPreview(ev.Args)
		if body != "" {
			fmt.Fprintln(r.Out, termcolor.ToolHeaderLine(label, body))
		} else {
			fmt.Fprintln(r.Out, termcolor.ToolHeaderLine(label, "…"))
		}
	case "completed":
		if preview := cursorNativeResultPreview(ev.Result); preview != "" {
			fmt.Fprintln(r.Out, termcolor.ToolHeaderLine(label, preview))
		} else {
			fmt.Fprintln(r.Out, termcolor.ToolHeaderLine(label, "done"))
		}
	case "error":
		msg := strings.TrimSpace(ev.Error)
		if msg == "" {
			msg = "failed"
		}
		fmt.Fprintln(r.Out, termcolor.WrapRed("cursor tool "+name+": "+msg))
	default:
		if body := cursorNativeArgsPreview(ev.Args); body != "" {
			fmt.Fprintln(r.Out, termcolor.ToolHeaderLine(label, body))
		}
	}
	flushWriter(r.Out)
}

func cursorNativeArgsPreview(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return truncateCursorPreview(string(raw), 160)
	}
	for _, key := range []string{"path", "command", "query", "pattern", "url", "task", "prompt", "description"} {
		if v, ok := m[key]; ok {
			if s := cursorJSONString(v); s != "" {
				return s
			}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return truncateCursorPreview(string(b), 160)
}

func cursorNativeResultPreview(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return truncateCursorPreview(string(raw), 200)
	}
	for _, key := range []string{"output", "content", "text", "message", "stdout", "stderr"} {
		if v, ok := m[key]; ok {
			if s := cursorJSONString(v); s != "" {
				return truncateCursorPreview(s, 200)
			}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return truncateCursorPreview(string(b), 200)
}

func cursorJSONString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func truncateCursorPreview(s string, max int) string {
	s = strings.TrimSpace(s)
	if max < 1 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
