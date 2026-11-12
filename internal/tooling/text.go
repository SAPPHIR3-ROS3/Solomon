package tooling

import "strings"

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
