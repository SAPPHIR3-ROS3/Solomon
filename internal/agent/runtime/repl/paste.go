package repl

import (
	"fmt"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TryPasteImageAtCursor(stderr io.Writer, saveImage func() (seq int, err error), line []rune, pos int, key rune) ([]rune, int, bool) {
	if key != rune(multiline.PasteImageKey) {
		return nil, 0, false
	}
	if !clipboard.HasImage() {
		return nil, 0, false
	}
	seq, err := saveImage()
	if err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "clipboard image paste failed: %v\n", err)
		}
		return nil, 0, false
	}
	line, pos = stripPasteTrigger(line, pos, key)
	tag := llm.ImagePlaceholder(seq)
	newRunes := make([]rune, 0, len(line)+len(tag))
	newRunes = append(newRunes, line[:pos]...)
	newRunes = append(newRunes, []rune(tag)...)
	newRunes = append(newRunes, line[pos:]...)
	return newRunes, pos + len([]rune(tag)), true
}
