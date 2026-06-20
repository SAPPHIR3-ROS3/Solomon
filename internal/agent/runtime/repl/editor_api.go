package repl

import (
	"bufio"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/editor"
)

var ErrInputInterrupted = editor.ErrInputInterrupted

type InputHistoryTest = editor.HistoryTest

func NewInputHistoryForTest() *InputHistoryTest {
	return editor.NewHistoryForTest()
}

type MultilineEditorTest = editor.MultilineEditorTest

func NewMultilineEditorForTest(loop *Loop, history *InputHistoryTest, lines []string, row, col, width int) *MultilineEditorTest {
	host := editor.Host{}
	if loop != nil {
		host = editorHostFromLoop(loop)
	}
	return editor.NewMultilineEditorForTest(host, history, lines, row, col, width)
}

func CommonRunePrefixForTest(candidates [][]rune) []rune {
	return editor.CommonRunePrefixForTest(candidates)
}

func ShellPrefixNormalizedForTest(buffer string, shellFirst bool) string {
	return editor.ShellPrefixNormalizedForTest(buffer, shellFirst)
}

func VisualRowsWithGhostForTest(width int, prompt, line, ghost string) int {
	return editor.VisualRowsWithGhostForTest(width, prompt, line, ghost)
}

func ReadInputRuneForTest(r *bufio.Reader) (rune, error) {
	return editor.ReadInputRuneForTest(r)
}

func editorHostFromLoop(loop *Loop) editor.Host {
	return editor.Host{
		RL:                     loop.RL,
		Out:                    loop.Out,
		InputInterrupt:         loop.InputInterrupt,
		CompleteEnv:            loop.CompleteEnv,
		PromptPrimary:          loop.PromptPrimary,
		PromptContinue:         loop.PromptContinue,
		ClipboardPasteForStdin: loop.ClipboardPasteForStdin,
	}
}
