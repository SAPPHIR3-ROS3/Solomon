package agentruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

func formatToolDisplayLines(name string, rawArgs json.RawMessage) []string {
	m := parseToolArgs(rawArgs)
	switch name {
	case "shell":
		return formatShellToolLines(m)
	case "readFile":
		return []string{termcolor.ToolHeaderLine("readFile", jsonString(m["path"]))}
	case "editFile":
		return formatEditFileToolLines(m)
	case "subagent":
		return formatSubagentToolLines(m)
	case "loadSkill":
		return []string{termcolor.ToolHeaderLine("loadSkill", jsonString(m["name"]))}
	case "searchSkill":
		return []string{termcolor.ToolHeaderLine("searchSkill", jsonString(m["query"]))}
	case "fetchWeb":
		return formatFetchWebToolLines(m)
	case "webSearch":
		return formatWebSearchToolLines(m)
	default:
		return []string{termcolor.WrapTool(fallbackToolLine(name, rawArgs))}
	}
}

func toolIntentLine(intent string) string {
	return termcolor.WrapThinking(intent)
}

func prependIntentLine(m map[string]json.RawMessage, lines []string) []string {
	intent := strings.TrimSpace(jsonString(m["intent"]))
	if intent == "" {
		return lines
	}
	return append([]string{toolIntentLine(intent)}, lines...)
}

func formatShellToolLines(m map[string]json.RawMessage) []string {
	body := jsonString(m["command"])
	if t := jsonIntPtr(m["timeoutSeconds"]); t != nil {
		body += fmt.Sprintf(" • %ds", *t)
	}
	return prependIntentLine(m, []string{termcolor.ToolHeaderLine("shell", body)})
}

func formatEditFileToolLines(m map[string]json.RawMessage) []string {
	return prependIntentLine(m, []string{
		termcolor.ToolHeaderLine("editFile", jsonString(m["path"])),
		termcolor.WrapTool(jsonString(m["oldString"])),
		termcolor.WrapTool(jsonString(m["newString"])),
	})
}

func formatSubagentToolLines(m map[string]json.RawMessage) []string {
	return []string{
		termcolor.ToolHeaderLine("subagent", "• "+jsonString(m["sysPromptPath"])),
		termcolor.ToolHeaderLine("subagent", jsonString(m["task"])),
	}
}

func formatFetchWebToolLines(m map[string]json.RawMessage) []string {
	body := jsonString(m["url"])
	if t := jsonIntPtr(m["timeoutSeconds"]); t != nil {
		body += fmt.Sprintf(" [• %d]", *t)
	}
	return []string{termcolor.ToolHeaderLine("fetchWeb", body)}
}

func formatWebSearchToolLines(m map[string]json.RawMessage) []string {
	body := jsonString(m["query"])
	if s := jsonString(m["engine"]); s != "" {
		body += " [• " + s + "]"
	}
	if n := jsonIntPtr(m["maxResults"]); n != nil {
		body += fmt.Sprintf(" [• %d]", *n)
	}
	if t := jsonIntPtr(m["timeoutSeconds"]); t != nil {
		body += fmt.Sprintf(" [• %d]", *t)
	}
	lines := []string{termcolor.ToolHeaderLine("webSearch", body)}
	if ex := formatExtrasLine(m["extras"]); ex != "" {
		lines = append(lines, termcolor.ToolHeaderLine("webSearch", ex))
	}
	return lines
}

func formatExtrasLine(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return buf.String()
}

func fallbackToolLine(name string, rawArgs json.RawMessage) string {
	s := string(rawArgs)
	if len(rawArgs) > 0 && json.Valid(rawArgs) {
		var buf bytes.Buffer
		if err := json.Compact(&buf, rawArgs); err == nil {
			s = buf.String()
		}
	}
	return fmt.Sprintf("Tool: %s(%s)", name, s)
}

func parseToolArgs(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]json.RawMessage{}
	}
	return m
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

func jsonIntPtr(raw json.RawMessage) *int {
	if len(raw) == 0 {
		return nil
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return &n
	}
	return nil
}

func formatToolPlainLines(name string, rawArgs json.RawMessage) []string {
	colored := formatToolDisplayLines(name, rawArgs)
	out := make([]string, len(colored))
	for i, line := range colored {
		out[i] = stripANSI(line)
	}
	return out
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEsc := false
	for i := 0; i < len(s); i++ {
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
