package tooling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func WriteToolDisplayLines(out io.Writer, cpSeq int, branchKey string, lines []string) {
	first := true
	cont := termcolor.WrapUserReadline("..... ")
	for _, line := range lines {
		parts := strings.Split(line, "\n")
		for _, part := range parts {
			if first {
				fmt.Fprintf(out, "%s%s\n", checkpoint.FormatCheckpointPrefix(cpSeq, branchKey), part)
			} else {
				fmt.Fprintf(out, "%s%s\n", cont, part)
			}
			first = false
		}
	}
}

func FormatToolDisplayLines(name string, rawArgs json.RawMessage) []string {
	m := parseToolDisplayArgs(rawArgs)
	switch name {
	case "shell":
		return formatShellToolDisplayLines(m)
	case "readFile":
		return []string{termcolor.ToolHeaderLine("readFile", jsonDisplayString(m["path"]))}
	case "find":
		return formatFindToolDisplayLines(m)
	case "editFile":
		return formatEditFileToolDisplayLines(m)
	case "subagent":
		return formatSubagentToolDisplayLines(m)
	case "loadSkill":
		return []string{termcolor.ToolHeaderLine("loadSkill", jsonDisplayString(m["name"]))}
	case "searchSkill":
		return []string{termcolor.ToolHeaderLine("searchSkill", jsonDisplayString(m["query"]))}
	case "fetchWeb":
		return formatFetchWebToolDisplayLines(m)
	case "webSearch":
		return formatWebSearchToolDisplayLines(m)
	default:
		return []string{termcolor.WrapTool(fallbackToolDisplayLine(name, rawArgs))}
	}
}

func ExtractToolIntent(rawArgs json.RawMessage) string {
	m := parseToolDisplayArgs(rawArgs)
	return strings.TrimSpace(jsonDisplayString(m["intent"]))
}

func formatFindToolDisplayLines(m map[string]json.RawMessage) []string {
	body := jsonDisplayString(m["pattern"])
	if jsonDisplayBool(m["files"]) {
		body = "files • " + body
	} else {
		body = "text • " + body
	}
	if p := jsonDisplayString(m["path"]); p != "" && p != "." {
		body += " • " + p
	}
	return []string{termcolor.ToolHeaderLine("find", body)}
}

func jsonDisplayBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var b bool
	return json.Unmarshal(raw, &b) == nil && b
}

func formatShellToolDisplayLines(m map[string]json.RawMessage) []string {
	body := jsonDisplayString(m["command"])
	if t := jsonDisplayIntPtr(m["timeoutSeconds"]); t != nil {
		body += fmt.Sprintf(" • %ds", *t)
	}
	return []string{termcolor.ToolHeaderLine("shell", body)}
}

func formatEditFileToolDisplayLines(m map[string]json.RawMessage) []string {
	return []string{
		termcolor.ToolHeaderLine("editFile", jsonDisplayString(m["path"])),
		termcolor.WrapTool(jsonDisplayString(m["oldString"])),
		termcolor.WrapTool(jsonDisplayString(m["newString"])),
	}
}

func formatSubagentToolDisplayLines(m map[string]json.RawMessage) []string {
	return []string{
		termcolor.ToolHeaderLine("subagent", "• "+jsonDisplayString(m["sysPromptPath"])),
		termcolor.ToolHeaderLine("subagent", jsonDisplayString(m["task"])),
	}
}

func formatFetchWebToolDisplayLines(m map[string]json.RawMessage) []string {
	body := jsonDisplayString(m["url"])
	if t := jsonDisplayIntPtr(m["timeoutSeconds"]); t != nil {
		body += fmt.Sprintf(" [• %d]", *t)
	}
	return []string{termcolor.ToolHeaderLine("fetchWeb", body)}
}

func formatWebSearchToolDisplayLines(m map[string]json.RawMessage) []string {
	body := jsonDisplayString(m["query"])
	if s := jsonDisplayString(m["engine"]); s != "" {
		body += " [• " + s + "]"
	}
	if n := jsonDisplayIntPtr(m["maxResults"]); n != nil {
		body += fmt.Sprintf(" [• %d]", *n)
	}
	if t := jsonDisplayIntPtr(m["timeoutSeconds"]); t != nil {
		body += fmt.Sprintf(" [• %d]", *t)
	}
	lines := []string{termcolor.ToolHeaderLine("webSearch", body)}
	if ex := formatToolDisplayExtrasLine(m["extras"]); ex != "" {
		lines = append(lines, termcolor.ToolHeaderLine("webSearch", ex))
	}
	return lines
}

func formatToolDisplayExtrasLine(raw json.RawMessage) string {
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

func fallbackToolDisplayLine(name string, rawArgs json.RawMessage) string {
	s := string(rawArgs)
	if len(rawArgs) > 0 && json.Valid(rawArgs) {
		var buf bytes.Buffer
		if err := json.Compact(&buf, rawArgs); err == nil {
			s = buf.String()
		}
	}
	return fmt.Sprintf("Tool: %s(%s)", name, s)
}

func parseToolDisplayArgs(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]json.RawMessage{}
	}
	return m
}

func jsonDisplayString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

func jsonDisplayIntPtr(raw json.RawMessage) *int {
	if len(raw) == 0 {
		return nil
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return &n
	}
	return nil
}
