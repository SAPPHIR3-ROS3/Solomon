package replhl

import (
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func forceSpan(styles []termcolor.ZshStyleKey, start, end int, key termcolor.ZshStyleKey) {
	if start < 0 {
		start = 0
	}
	if end > len(styles) {
		end = len(styles)
	}
	for i := start; i < end; i++ {
		styles[i] = key
	}
}

func renderStyled(line string, styles []termcolor.ZshStyleKey) string {
	if len(line) == 0 {
		return ""
	}
	if len(styles) < len(line) {
		styles = append(styles, make([]termcolor.ZshStyleKey, len(line)-len(styles))...)
	}
	var b strings.Builder
	b.Grow(len(line) * 8)
	i := 0
	for i < len(line) {
		key := styles[i]
		if key == "" {
			key = termcolor.ZshDefault
		}
		j := i + 1
		for j < len(line) && styles[j] == key {
			j++
		}
		b.WriteString(termcolor.ZshStyle(key, line[i:j]))
		i = j
	}
	return b.String()
}
