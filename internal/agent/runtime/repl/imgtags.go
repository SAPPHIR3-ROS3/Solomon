package repl

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

type imgDisplayPainter struct{}

func (imgDisplayPainter) Paint(line []rune, _ int) []rune {
	return []rune(termcolor.ColorizeReplInputTags(string(line)))
}

func stripPasteTrigger(line []rune, pos int, key rune) ([]rune, int) {
	if pos <= 0 || key != rune(multiline.PasteImageKey) || line[pos-1] != rune(multiline.PasteImageKey) {
		return line, pos
	}
	return append(append([]rune(nil), line[:pos-1]...), line[pos:]...), pos - 1
}
