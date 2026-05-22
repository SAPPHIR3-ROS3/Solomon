package tooloutput

import "strings"

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
