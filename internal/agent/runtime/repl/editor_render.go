package repl

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func (e *multilineEditor) primaryPrompt() string {
	if e.loop.PromptPrimary != nil {
		return e.loop.PromptPrimary()
	}
	return "You: "
}

func (e *multilineEditor) continuePrompt() string {
	if e.loop.PromptContinue != nil {
		return e.loop.PromptContinue()
	}
	return ".... "
}

func (e *multilineEditor) promptFor(row int) string {
	if row == 0 {
		return e.primaryPrompt()
	}
	return e.continuePrompt()
}

func (e *multilineEditor) refresh() {
	e.moveToTop()
	for i := 0; i < e.rendered; i++ {
		fmt.Fprint(e.out, "\x1b[2K")
		if i+1 < e.rendered {
			fmt.Fprint(e.out, "\x1b[B\r")
		}
	}
	if e.rendered > 1 {
		fmt.Fprintf(e.out, "\x1b[%dA\r", e.rendered-1)
	}
	rows := e.render()
	e.rendered = rows
	e.moveToCursor()
}

func (e *multilineEditor) moveToTop() {
	if e.cursorLine > 0 {
		fmt.Fprintf(e.out, "\x1b[%dA", e.cursorLine)
	}
	fmt.Fprint(e.out, "\r")
}

func (e *multilineEditor) render() int {
	rows := 0
	for i, line := range e.lines {
		if i > 0 {
			fmt.Fprint(e.out, "\r\n")
		}
		prompt := e.promptFor(i)
		fmt.Fprint(e.out, prompt)
		fmt.Fprint(e.out, termcolor.ColorizeImgTagsReplInput(string(line)))
		rows += e.visualRows(prompt, line)
	}
	if rows == 0 {
		return 1
	}
	return rows
}

func (e *multilineEditor) moveToCursor() {
	cursorRow, cursorCol := e.cursorVisual()
	if e.rendered-1 > cursorRow {
		fmt.Fprintf(e.out, "\x1b[%dA", e.rendered-1-cursorRow)
	}
	fmt.Fprint(e.out, "\r")
	if cursorCol > 0 {
		fmt.Fprintf(e.out, "\x1b[%dC", cursorCol)
	}
	e.cursorLine = cursorRow
}

func (e *multilineEditor) finish() {
	e.moveToTop()
	if e.rendered > 0 {
		fmt.Fprintf(e.out, "\x1b[%dB\r\n", e.rendered-1)
	}
	e.rendered = 0
	e.cursorLine = 0
}

func (e *multilineEditor) cursorVisual() (int, int) {
	row := 0
	for i := 0; i < e.row; i++ {
		row += e.visualRows(e.promptFor(i), e.lines[i])
	}
	cells := visibleCells(e.promptFor(e.row)) + runesCells(e.lines[e.row][:e.col])
	if e.width <= 0 {
		return row, cells
	}
	return row + cells/e.width, cells % e.width
}

func (e *multilineEditor) visualRows(prompt string, line []rune) int {
	if e.width <= 0 {
		return 1
	}
	cells := visibleCells(prompt) + runesCells(line)
	rows := cells / e.width
	if cells%e.width != 0 || rows == 0 {
		rows++
	}
	return rows
}

func runesCells(rs []rune) int {
	total := 0
	for _, r := range rs {
		total += runeDisplayWidth(r)
	}
	return total
}
