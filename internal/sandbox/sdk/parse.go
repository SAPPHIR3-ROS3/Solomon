package sdk

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func strField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func intField(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

func stringSliceField(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch arr := v.(type) {
	case []string:
		return append([]string(nil), arr...)
	case []any:
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func unmarshalToolMap(raw []byte) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func parseReadResult(m map[string]any) ReadResult {
	r := ReadResult{
		Path:       strField(m, "path"),
		Content:    strField(m, "content"),
		TotalLines: intField(m, "total_lines"),
		StartLine:  intField(m, "start_line"),
		EndLine:    intField(m, "end_line"),
	}
	if r.StartLine == 0 && r.EndLine == 0 && r.TotalLines > 0 {
		r.StartLine = 1
		r.EndLine = r.TotalLines
	}
	return r
}

func parseShellResult(m map[string]any) ShellOutput {
	return ShellOutput{
		Output: strField(m, "output"),
		Exit:   intField(m, "exit"),
		Intent: strField(m, "intent"),
	}
}

func boolField(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func parseGrepLines(output string) []GrepLine {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	out := make([]GrepLine, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		out = append(out, GrepLine{Path: parts[0], Line: n, Text: parts[2]})
	}
	return out
}

func parseGrepCountEntries(output string) []GrepCountEntry {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	out := make([]GrepCountEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		out = append(out, GrepCountEntry{Path: parts[0], Count: n})
	}
	return out
}

func parseFindResult(m map[string]any) FindResult {
	return FindResult{
		Files:      boolField(m, "files"),
		Pattern:    strField(m, "pattern"),
		Path:       strField(m, "path"),
		Matches:    stringSliceField(m, "matches"),
		Count:      intField(m, "count"),
		OutputMode: strField(m, "outputMode"),
		Output:     strField(m, "output"),
		Exit:       intField(m, "exit"),
	}
}

func parseEditResult(raw json.RawMessage) (EditResult, error) {
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return EditResult{}, err
	}
	return EditResult{
		OK:     boolField(m, "ok"),
		Action: strField(m, "action"),
		Reason: strField(m, "reason"),
		From:   strField(m, "from"),
		To:     strField(m, "to"),
		Intent: strField(m, "intent"),
	}, nil
}

func parseWebSearchResult(raw json.RawMessage) (WebSearchResult, error) {
	var r WebSearchResult
	if err := json.Unmarshal(raw, &r); err != nil {
		return WebSearchResult{}, err
	}
	return r, nil
}

func parseDocsResult(raw json.RawMessage) (DocsResult, error) {
	var r DocsResult
	if err := json.Unmarshal(raw, &r); err != nil {
		return DocsResult{}, err
	}
	return r, nil
}

func editErr(r EditResult) error {
	if r.OK {
		return nil
	}
	if r.Reason != "" {
		return fmt.Errorf("editFile: %s", r.Reason)
	}
	return fmt.Errorf("editFile failed")
}

func parseFetchWebResult(m map[string]any) FetchWebResult {
	return FetchWebResult{
		URL:         strField(m, "url"),
		Status:      intField(m, "status"),
		ContentType: strField(m, "contentType"),
		Markdown:    strField(m, "markdown"),
	}
}
