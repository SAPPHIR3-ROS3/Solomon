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

const (
	webSearchDisplayDefaultMaxResults = 10
	webSearchDisplayDefaultTimeoutS   = 30
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

func splitEditContentLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

func editDisplayLine(ln string) string {
	if ln == "" {
		return " "
	}
	return ln
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
	query := jsonDisplayString(m["query"])
	engine := jsonDisplayString(m["engine"])
	maxResults := webSearchDisplayDefaultMaxResults
	if n := jsonDisplayIntPtr(m["maxResults"]); n != nil {
		maxResults = *n
	}
	timeout := webSearchDisplayDefaultTimeoutS
	if t := jsonDisplayIntPtr(m["timeoutSeconds"]); t != nil {
		timeout = *t
	}
	meta := formatWebSearchMeta(engine, maxResults, timeout)
	lines := []string{termcolor.ToolLine("webSearch", meta) + termcolor.WrapThinking(query)}
	if ex := formatToolDisplayExtrasLine(m["extras"]); ex != "" {
		lines = append(lines, termcolor.ToolLine("webSearch", ex))
	}
	return lines
}

func formatWebSearchMeta(engine string, maxResults, timeout int) string {
	var parens string
	customMax := maxResults != webSearchDisplayDefaultMaxResults
	customTimeout := timeout != webSearchDisplayDefaultTimeoutS
	switch {
	case customMax && customTimeout:
		parens = fmt.Sprintf("(%d result • %ds)", maxResults, timeout)
	case customMax:
		parens = fmt.Sprintf("(%d result)", maxResults)
	case customTimeout:
		parens = fmt.Sprintf("(%ds)", timeout)
	}
	enginePart := ""
	if engine != "" {
		enginePart = engine + " "
	}
	if parens != "" {
		return fmt.Sprintf("• %s%s | ", enginePart, parens)
	}
	if enginePart != "" {
		return "• " + strings.TrimSpace(enginePart) + " | "
	}
	return "• | "
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
	if err := json.Unmarshal(raw, &m); err == nil && len(m) > 0 {
		return m
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && json.Unmarshal([]byte(s), &m) == nil && len(m) > 0 {
		return m
	}
	return map[string]json.RawMessage{}
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

func FormatToolResultDisplayLines(toolName string, payload string) []string {
	toolName = strings.TrimSpace(toolName)
	label := toolName
	if label == "" {
		label = "tool"
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return []string{termcolor.WrapThinking(truncateDisplayRunes(payload, 200))}
	}
	if body := formatToolResultBody(toolName, m); body != "" {
		return []string{termcolor.ToolHeaderLine(label, body)}
	}
	if toolResultDisplaySuppressed(toolName, m) {
		return nil
	}
	return []string{termcolor.WrapThinking(compactToolResultJSON(m, 200))}
}

func formatToolResultBody(toolName string, m map[string]json.RawMessage) string {
	if errMsg := jsonDisplayString(m["error"]); errMsg != "" {
		return "error: " + errMsg
	}
	switch toolName {
	case "readFile":
		path := jsonDisplayString(m["path"])
		if path == "" {
			path = "file"
		}
		body := "→ " + path
		if n, ok := jsonDisplayInt(m["total_lines"]); ok && n > 0 {
			body += fmt.Sprintf(" (%d lines)", n)
		}
		return body
	case "shell":
		exit := 0
		if n, ok := jsonDisplayInt(m["exit"]); ok {
			exit = n
		}
		body := fmt.Sprintf("→ exit %d", exit)
		if out := strings.TrimSpace(jsonDisplayString(m["output"])); out != "" {
			body += " • " + firstDisplayLine(out, 120)
		}
		return body
	case "editFile", "editPlan":
		if ok := jsonDisplayBool(m["ok"]); !ok {
			if r := jsonDisplayString(m["reason"]); r != "" {
				return "→ " + r
			}
			return "→ failed"
		}
		return ""
	case "find":
		if n, ok := jsonDisplayInt(m["matches"]); ok {
			return fmt.Sprintf("→ %d matches", n)
		}
		return "→ done"
	default:
		return formatGenericToolResultBody(m)
	}
}

func toolResultDisplaySuppressed(toolName string, m map[string]json.RawMessage) bool {
	switch toolName {
	case "editFile", "editPlan":
		if jsonDisplayString(m["error"]) != "" {
			return false
		}
		rawOK, hasOK := m["ok"]
		return hasOK && len(rawOK) > 0 && jsonDisplayBool(rawOK)
	default:
		return false
	}
}

func formatGenericToolResultBody(m map[string]json.RawMessage) string {
	rawOK, hasOK := m["ok"]
	if !hasOK || len(rawOK) == 0 {
		return ""
	}
	if !jsonDisplayBool(rawOK) {
		if r := jsonDisplayString(m["reason"]); r != "" {
			return "→ " + r
		}
		return "→ failed"
	}
	if a := jsonDisplayString(m["action"]); a != "" {
		return "→ " + a
	}
	return "→ ok"
}

func compactToolResultJSON(m map[string]json.RawMessage, maxRunes int) string {
	omit := map[string]bool{"content": true, "output": true}
	trimmed := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		if omit[k] {
			continue
		}
		trimmed[k] = v
	}
	if len(trimmed) == 0 {
		return "→ (large result omitted)"
	}
	b, err := json.Marshal(trimmed)
	if err != nil {
		return "→ (result)"
	}
	return "→ " + truncateDisplayRunes(string(b), maxRunes)
}

func jsonDisplayInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n, true
	}
	return 0, false
}

func firstDisplayLine(s string, maxRunes int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return truncateDisplayRunes(strings.TrimSpace(s), maxRunes)
}

func truncateDisplayRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
