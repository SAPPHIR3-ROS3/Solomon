package tooloutput

import "strings"

var toolPrimaryTextField = map[string]string{
	"readFile":    "content",
	"shell":       "output",
	"orchestrate": "output",
}

func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func exceedsLimits(text string, lim Limits) bool {
	if text == "" {
		return false
	}
	return len(text) > lim.MaxBytes || lineCount(text) > lim.MaxLines
}

func truncatePrimaryTextField(m map[string]any, toolName, field string, lim Limits, meta Meta, projectHex string) (map[string]any, bool) {
	raw, ok := m[field]
	if !ok || raw == nil {
		return m, false
	}
	text, ok := raw.(string)
	if !ok || !exceedsLimits(text, lim) {
		return m, false
	}
	out := cloneStringMap(m)
	spillPath, err := writeSpill(projectHex, meta.SessionID, meta.ToolCallID, meta.ToolName, []byte(text))
	if err != nil {
		out[field] = FormatTruncatedMessage("")
		out["truncated"] = true
		out["spill_error"] = err.Error()
		return out, true
	}
	out[field] = FormatTruncatedMessage(spillPath)
	out["truncated"] = true
	out["spill_path"] = spillPath
	return out, true
}

func cloneStringMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}
