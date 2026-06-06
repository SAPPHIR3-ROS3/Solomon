package tooling

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
)

const editFileDisplayHeadTail = 10

func formatEditFileToolDisplayLines(m map[string]json.RawMessage) []string {
	path := jsonDisplayString(m["path"])
	if jsonDisplayBool(m["delete"]) {
		return []string{termcolor.EditFileDeleteToolLine(path)}
	}
	if to := jsonDisplayString(m["renameTo"]); to != "" {
		return []string{termcolor.ToolHeaderLine("editFile", path+" → "+to)}
	}
	oldS := jsonDisplayString(m["oldString"])
	newS := jsonDisplayString(m["newString"])
	out := []string{termcolor.ToolHeaderLine("editFile", path)}
	removed, added := editFileDiffRemovedAdded(oldS, newS)
	out = append(out, formatEditFileDiffSide(removed, termcolor.WrapEditFileOldString)...)
	out = append(out, formatEditFileDiffSide(added, termcolor.WrapEditFileNewString)...)
	return out
}

func editFileDiffRemovedAdded(oldS, newS string) (removed, added []string) {
	oldLines := splitEditContentLines(oldS)
	newLines := splitEditContentLines(newS)
	prefix := editFileCommonPrefixLen(oldLines, newLines)
	suffix := editFileCommonSuffixLen(oldLines, newLines, prefix)
	if prefix+suffix <= len(oldLines) {
		removed = oldLines[prefix : len(oldLines)-suffix]
	}
	if prefix+suffix <= len(newLines) {
		added = newLines[prefix : len(newLines)-suffix]
	}
	return removed, added
}

func editFileCommonPrefixLen(a, b []string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func editFileCommonSuffixLen(a, b []string, prefix int) int {
	maxA := len(a) - prefix
	maxB := len(b) - prefix
	if maxA < 0 {
		maxA = 0
	}
	if maxB < 0 {
		maxB = 0
	}
	n := maxA
	if maxB < n {
		n = maxB
	}
	i := 0
	for i < n && a[len(a)-1-i] == b[len(b)-1-i] {
		i++
	}
	return i
}

func formatEditFileDiffSide(lines []string, wrap func(string) string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	for _, ln := range headTailTruncateEditLines(lines) {
		if ln == tooloutput.TruncatedMarker {
			out = append(out, termcolor.WrapThinking(tooloutput.TruncatedMarker))
			continue
		}
		out = append(out, wrap(editDisplayLine(ln)))
	}
	return out
}

func headTailTruncateEditLines(lines []string) []string {
	max := editFileDisplayHeadTail * 2
	if len(lines) <= max {
		return lines
	}
	out := make([]string, 0, max+1)
	out = append(out, lines[:editFileDisplayHeadTail]...)
	out = append(out, tooloutput.TruncatedMarker)
	out = append(out, lines[len(lines)-editFileDisplayHeadTail:]...)
	return out
}
