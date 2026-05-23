package agentruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

const legacyToolJSONCorrectionUserMsg = "Your previous reply contained a malformed <tool_calls> block. Use exactly this shape with valid JSON in each <args> tag:\n<tool_calls>\n<tool name=\"TOOL_NAME\">\n<intent>brief purpose</intent>\n<args>{\"key\":\"value\"}</args>\n</tool>\n</tool_calls>\nSend a corrected block only, or continue without tools if you meant plain text."

func newLegacyStreamWriter(out io.Writer, enabled bool, allowed map[string]struct{}) (*tooling.LegacyStreamWriter, io.Writer) {
	if !enabled {
		return nil, out
	}
	lsw := tooling.NewLegacyStreamWriter(out, formatToolDisplayLines, allowed)
	return lsw, lsw
}

const legacyNativeToolRejectedUserMsg = "Native API tool_calls are disabled because legacy tools force is ON. Do not use function calling. Emit tool invocations only inside a <tool_calls> XML block as described in the system prompt."

func (r *Runtime) handleRejectedNativeToolCall() error {
	if !r.machineMode() {
		termcolor.WriteSystem(r.Out, "Legacy tools force: native API tool_calls were ignored. Use <tool_calls> XML in assistant text.")
		fmt.Fprintln(r.Out)
		flushWriter(r.Out)
	}
	r.mutateSession(func(s *chatstore.Session) {
		seq := checkpoint.Bump(s)
		um := chatstore.Message{Role: "user", Content: legacyNativeToolRejectedUserMsg}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		s.LastMessageAt = time.Now()
	})
	return r.persistSession()
}

func (r *Runtime) handleMalformedLegacyTool(err error) error {
	if !r.machineMode() {
		termcolor.WriteSystem(r.Out, legacyToolScreenMessage(err))
		fmt.Fprintln(r.Out)
		flushWriter(r.Out)
	}
	r.mutateSession(func(s *chatstore.Session) {
		seq := checkpoint.Bump(s)
		um := chatstore.Message{Role: "user", Content: legacyToolJSONCorrectionUserMsg}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		s.LastMessageAt = time.Now()
	})
	return r.persistSession()
}

func legacyToolScreenMessage(err error) string {
	msg := "Legacy tool syntax error: <tool_calls> block is invalid."
	if err != nil {
		msg = "Legacy tool syntax error: " + err.Error()
	}
	return msg + " Use <tool_calls> with <tool name=\"...\">, optional <intent>, and <args>{...}</args>."
}

func isMalformedLegacyToolErr(err error) bool {
	return errors.Is(err, tooling.ErrMalformedLegacyTool) || errors.Is(err, tooling.ErrUnknownLegacyTool)
}

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
