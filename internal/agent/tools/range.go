package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const editFileDescription = "Replace oldString once with newString; for exact text replacements prefer sdk.ReplaceInFile. Optional startLine/endLine select existing whole lines (1-based, inclusive; omitted bounds default to first/last line). In range mode, newString is required and non-empty oldString checks the selected content before editing. Without a range, empty oldString creates or overwrites. Use delete=true or renameTo only without a range."

func validateEditRange(a editArgs, fields map[string]json.RawMessage) (bool, error) {
	_, startProvided := fields["startLine"]
	_, endProvided := fields["endLine"]
	if !startProvided && !endProvided {
		return false, nil
	}
	if (startProvided && a.StartLine == nil) || (endProvided && a.EndLine == nil) {
		return true, fmt.Errorf("editFile range bounds must be integers, not null")
	}
	if a.Delete || strings.TrimSpace(a.RenameTo) != "" {
		return true, fmt.Errorf("editFile range cannot be combined with delete or renameTo")
	}
	if _, ok := fields["newString"]; !ok || strings.TrimSpace(string(fields["newString"])) == "null" {
		return true, fmt.Errorf("editFile range requires an explicit newString; use an empty string to remove the selected lines")
	}
	if (a.StartLine != nil && *a.StartLine < 1) || (a.EndLine != nil && *a.EndLine < 1) {
		return true, fmt.Errorf("editFile range bounds must be at least 1")
	}
	return true, nil
}

func execEditRange(env *Env, path string, a editArgs) (any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitAfter(string(content), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	start, end := 1, len(lines)
	if a.StartLine != nil {
		start = *a.StartLine
	}
	if a.EndLine != nil {
		end = *a.EndLine
	}
	if start > end || start > len(lines) || end > len(lines) {
		return nil, fmt.Errorf("editFile range %d..%d is outside the file (%d lines); read the current lines before retrying", start, end, len(lines))
	}
	selected := strings.Join(lines[start-1:end], "")
	withoutLF := strings.TrimSuffix(selected, "\n")
	withoutEnding := strings.TrimSuffix(selected, "\r\n")
	if a.OldString != "" && a.OldString != selected && a.OldString != withoutLF && a.OldString != withoutEnding {
		return map[string]any{"ok": false, "reason": "oldString does not match the selected lines; read the current file before retrying"}, nil
	}
	replacement := a.NewString
	if replacement != "" && strings.HasSuffix(selected, "\n") && !strings.HasSuffix(replacement, "\n") {
		ending := "\n"
		if strings.HasSuffix(selected, "\r\n") {
			ending = "\r\n"
		}
		if ending == "\r\n" && strings.HasSuffix(replacement, "\r") {
			replacement += "\n"
		} else {
			replacement += ending
		}
	}
	updated := strings.Join(lines[:start-1], "") + replacement + strings.Join(lines[end:], "")
	stageBeforeEdit(env, path)
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		return nil, err
	}
	stageAfterEdit(env, "patch", path, "")
	stageProjectPath(env, path)
	return map[string]any{
		"ok": true, "action": "edited", "start_line": start, "end_line": end, "intent": a.Intent,
	}, nil
}
