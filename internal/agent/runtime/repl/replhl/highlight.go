package replhl

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
)

func HighlightInputLine(lines [][]rune, row int, shellFirst bool, env replcomplete.ReplCompleteEnv) string {
	if row < 0 || row >= len(lines) {
		return ""
	}
	full := string(lines[row])
	switch classifyBufferLine(lines, row, shellFirst) {
	case inputSlash:
		return HighlightSlash(full, env)
	case inputShell:
		off := shellHighlightOffset(lines, row, shellFirst)
		shellText := shellLineText(lines, row, shellFirst)
		return HighlightShellBufferLine(full, shellText, off, ShellEnv{ProjRoot: env.ProjRoot})
	default:
		return full
	}
}
