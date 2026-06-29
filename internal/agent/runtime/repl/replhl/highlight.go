package replhl

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func HighlightInputLine(lines [][]rune, row int, shellFirst bool, env replcomplete.ReplCompleteEnv) string {
	return HighlightInputLineSlice(lines, row, 0, len(lines[row]), shellFirst, env)
}

func HighlightInputLineSlice(lines [][]rune, row, start, end int, shellFirst bool, env replcomplete.ReplCompleteEnv) string {
	if row < 0 || row >= len(lines) {
		return ""
	}
	rs := lines[row]
	if start < 0 {
		start = 0
	}
	if end > len(rs) {
		end = len(rs)
	}
	if start >= end {
		return ""
	}
	full := string(rs)
	switch classifyBufferLine(lines, row, shellFirst) {
	case inputSlash:
		styles := make([]termcolor.ZshStyleKey, len(rs))
		applySlashLexical(rs, styles)
		applySlashSemantic(rs, styles, env)
		return renderStyled(string(rs[start:end]), styles[start:end])
	case inputShell:
		if start == 0 && end == len(rs) {
			off := shellHighlightOffset(lines, row, shellFirst)
			shellText := shellLineText(lines, row, shellFirst)
			return HighlightShellBufferLine(full, shellText, off, ShellEnv{ProjRoot: env.ProjRoot})
		}
		return string(rs[start:end])
	default:
		styles := make([]termcolor.ZshStyleKey, len(rs))
		applySlashSemantic(rs, styles, env)
		return renderStyled(string(rs[start:end]), styles[start:end])
	}
}
