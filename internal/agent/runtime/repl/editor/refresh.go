package editor

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/replhl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const replGhostANSI = "\x1b[2m\x1b[90m"

type visualRow struct {
	line           int
	start          int
	end            int
	prompt         string
	picker         string
	pickerSelected bool
	ghostOnly      string
}

func (e *multilineEditor) primaryPrompt() string {
	if e.host.PromptPrimary != nil {
		return e.host.PromptPrimary()
	}
	return "You: "
}

func (e *multilineEditor) continuePrompt() string {
	if e.host.PromptContinue != nil {
		return e.host.PromptContinue()
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
	allRows := e.buildVisualRows()
	cursorRow, cursorCol := e.cursorVisualInRows(allRows)
	top := e.viewportStart(cursorRow, len(allRows))
	visibleRows := e.visibleRows(allRows, top)

	e.moveToTop()
	rows := e.renderRows(visibleRows)
	e.clearRenderedTail(rows)
	e.rendered = rows
	e.viewportTop = top
	e.moveToCursor(cursorRow-top, cursorCol)
}

func (e *multilineEditor) clearRenderedTail(newRows int) {
	if e.rendered <= newRows {
		return
	}
	for i := newRows; i < e.rendered; i++ {
		fmt.Fprint(e.out, "\r\n\x1b[2K")
	}
	if e.rendered-newRows > 0 {
		fmt.Fprintf(e.out, "\x1b[%dA\r", e.rendered-newRows)
	}
}

func (e *multilineEditor) ghostParts() []string {
	if !e.cursorAtBufferEnd() || len(e.suggestSuffix) == 0 {
		return nil
	}
	return strings.Split(string(e.suggestSuffix), "\n")
}

func (e *multilineEditor) totalContentVisualRows() int {
	return len(e.buildVisualRows())
}

func (e *multilineEditor) buildVisualRows() []visualRow {
	ghostParts := e.ghostParts()
	rows := make([]visualRow, 0, len(e.lines))
	lineCount := len(e.lines)
	for i, line := range e.lines {
		combined := append([]rune(nil), line...)
		if i == lineCount-1 && len(ghostParts) > 0 {
			combined = append(combined, []rune(ghostParts[0])...)
		}
		rows = append(rows, e.wrapVisualRows(i, e.promptFor(i), combined)...)
	}
	for i := range e.atMatches {
		if e.atPickerItemVisible(i) {
			marker := "  "
			if i == e.atSelected {
				marker = "> "
			}
			rows = append(rows, visualRow{
				line:           -1,
				picker:         marker + e.atMatches[i].RelPath,
				pickerSelected: i == e.atSelected,
			})
		}
	}
	for gi := 1; gi < len(ghostParts); gi++ {
		for _, row := range e.wrapVisualRows(-1, e.continuePrompt(), []rune(ghostParts[gi])) {
			row.ghostOnly = ghostParts[gi]
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return []visualRow{{line: 0, prompt: e.primaryPrompt()}}
	}
	return rows
}

func (e *multilineEditor) wrapVisualRows(line int, prompt string, text []rune) []visualRow {
	if len(text) == 0 {
		return []visualRow{{line: line, prompt: prompt}}
	}
	rows := make([]visualRow, 0, e.visualRows(prompt, text))
	start := 0
	first := true
	for start < len(text) {
		rowPrompt := ""
		if first {
			rowPrompt = prompt
		}
		end := visualRowEnd(text, start, e.textCellsFor(rowPrompt))
		rows = append(rows, visualRow{line: line, start: start, end: end, prompt: rowPrompt})
		start = end
		first = false
	}
	return rows
}

func visualRowEnd(text []rune, start, maxCells int) int {
	if maxCells < 1 {
		maxCells = 1
	}
	cells := 0
	for i := start; i < len(text); i++ {
		w := runeDisplayWidth(text[i])
		if i > start && cells+w > maxCells {
			return i
		}
		cells += w
	}
	return len(text)
}

func (e *multilineEditor) textCellsFor(prompt string) int {
	if e.width <= 1 {
		return 1
	}
	cols := e.width - 1 - visibleCells(prompt)
	if cols < 1 {
		return 1
	}
	return cols
}

func (e *multilineEditor) maxRenderedRows() int {
	if e.height <= 0 {
		return len(e.buildVisualRows())
	}
	if e.height <= 4 {
		return e.height
	}
	return e.height - 1
}

func (e *multilineEditor) viewportStart(cursorRow, totalRows int) int {
	maxRows := e.maxRenderedRows()
	if maxRows <= 0 || totalRows <= maxRows {
		return 0
	}
	top := e.viewportTop
	if cursorRow < top {
		top = cursorRow
	}
	if cursorRow >= top+maxRows {
		top = cursorRow - maxRows + 1
	}
	if e.atPickerActive() && e.cursorAtBufferEnd() {
		bottom := totalRows - 1
		if bottom >= top+maxRows {
			top = bottom - maxRows + 1
			if top > cursorRow {
				top = cursorRow
			}
		}
	}
	if top < 0 {
		return 0
	}
	maxTop := totalRows - maxRows
	if top > maxTop {
		return maxTop
	}
	return top
}

func (e *multilineEditor) visibleRows(rows []visualRow, top int) []visualRow {
	if top < 0 {
		top = 0
	}
	if top >= len(rows) {
		return nil
	}
	maxRows := e.maxRenderedRows()
	if maxRows <= 0 || top+maxRows > len(rows) {
		return rows[top:]
	}
	return rows[top : top+maxRows]
}

func (e *multilineEditor) renderRows(rows []visualRow) int {
	ghostParts := e.ghostParts()
	for i, row := range rows {
		if i > 0 {
			fmt.Fprint(e.out, "\r\n")
		}
		fmt.Fprint(e.out, "\x1b[2K\r")
		if row.picker != "" {
			fmt.Fprint(e.out, replGhostANSI)
			fmt.Fprint(e.out, row.picker)
			fmt.Fprint(e.out, "\x1b[0m")
			continue
		}
		fmt.Fprint(e.out, row.prompt)
		if row.ghostOnly != "" {
			fmt.Fprint(e.out, replGhostANSI)
			fmt.Fprint(e.out, termcolor.ColorizeReplInputTags(string([]rune(row.ghostOnly)[row.start:row.end])))
			fmt.Fprint(e.out, "\x1b[0m")
			continue
		}
		if row.line < 0 || row.line >= len(e.lines) {
			continue
		}
		line := e.lines[row.line]
		lineEnd := row.end
		if lineEnd > len(line) {
			lineEnd = len(line)
		}
		if row.start < lineEnd {
			lineStr := replhl.HighlightInputLineSlice(e.lines, row.line, row.start, lineEnd, e.host.CompleteEnv.ReplShellFirst, e.host.CompleteEnv)
			fmt.Fprint(e.out, termcolor.ColorizeReplInputTags(lineStr))
		}
		if len(ghostParts) > 0 && row.line == len(e.lines)-1 && row.end > len(line) {
			ghost := []rune(ghostParts[0])
			ghostStart := row.start - len(line)
			if ghostStart < 0 {
				ghostStart = 0
			}
			ghostEnd := row.end - len(line)
			if ghostEnd > len(ghost) {
				ghostEnd = len(ghost)
			}
			if ghostStart < ghostEnd {
				fmt.Fprint(e.out, replGhostANSI)
				fmt.Fprint(e.out, termcolor.ColorizeReplInputTags(string(ghost[ghostStart:ghostEnd])))
				fmt.Fprint(e.out, "\x1b[0m")
			}
		}
	}
	if len(rows) == 0 {
		return 1
	}
	return len(rows)
}

func (e *multilineEditor) moveToTop() {
	if e.cursorLine > 0 {
		fmt.Fprintf(e.out, "\x1b[%dA", e.cursorLine)
	}
	fmt.Fprint(e.out, "\r")
}

func (e *multilineEditor) moveToCursor(cursorRow, cursorCol int) {
	if cursorRow < 0 {
		cursorRow = 0
	}
	if cursorRow >= e.rendered {
		cursorRow = e.rendered - 1
	}
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
	redrewFull := false
	allRows := e.buildVisualRows()
	if e.viewportTop > 0 || len(allRows) > e.rendered {
		e.rendered = e.renderRows(allRows)
		e.cursorLine = e.rendered - 1
		e.viewportTop = 0
		redrewFull = true
	}
	if e.rendered > 0 {
		if !redrewFull && e.rendered > 1 {
			fmt.Fprintf(e.out, "\x1b[%dB", e.rendered-1)
		}
		fmt.Fprint(e.out, "\r\n")
	}
	e.rendered = 0
	e.cursorLine = 0
	e.viewportTop = 0
}

func (e *multilineEditor) cursorVisualInRows(rows []visualRow) (int, int) {
	for i, row := range rows {
		if row.line != e.row {
			continue
		}
		lineLen := len(e.lines[e.row])
		end := row.end
		if end > lineLen {
			end = lineLen
		}
		if e.col < row.start || e.col > end {
			continue
		}
		cells := visibleCells(row.prompt) + runesCells(e.lines[e.row][row.start:e.col])
		return i, cells
	}
	return 0, visibleCells(e.promptFor(e.row))
}

func (e *multilineEditor) visualRows(prompt string, line []rune) int {
	if len(line) == 0 {
		return 1
	}
	rows := 0
	start := 0
	first := true
	for start < len(line) {
		rowPrompt := ""
		if first {
			rowPrompt = prompt
		}
		start = visualRowEnd(line, start, e.textCellsFor(rowPrompt))
		rows++
		first = false
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
