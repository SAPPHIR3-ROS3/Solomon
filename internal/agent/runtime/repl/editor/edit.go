package editor

import (
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func (e *multilineEditor) insertPaste(s string) {
	e.wrapDisabled = true
	if s == "" {
		if e.host.ClipboardPasteForStdin != nil {
			if tag, ok := e.host.ClipboardPasteForStdin(); ok {
				e.insertString(tag)
			}
		}
		e.wrapDisabled = false
		return
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	e.insertString(s)
	e.wrapDisabled = false
}

func (e *multilineEditor) insertString(s string) {
	for _, r := range s {
		if r == '\n' {
			e.insertNewline()
		} else {
			e.insertRuneRaw(r)
		}
	}
}

func (e *multilineEditor) insertRuneRaw(r rune) {
	line := e.lines[e.row]
	line = append(line[:e.col], append([]rune{r}, line[e.col:]...)...)
	e.lines[e.row] = line
	e.col++
}

func (e *multilineEditor) insertRune(r rune) {
	if !e.wrapDisabled && r != '\n' && e.col == len(e.lines[e.row]) && e.width > 0 {
		prompt := e.promptFor(e.row)
		cells := visibleCells(prompt) + runesCells(e.lines[e.row]) + runeDisplayWidth(r)
		if cells > e.width {
			splitAt := e.findWrapSplit(prompt)
			if splitAt >= 0 {
				e.wrapClearPrevRow = true
				e.col = splitAt + 1
				e.insertNewline()
				e.col = len(e.lines[e.row])
				e.insertRuneRaw(r)
				e.suggestSuffix = nil
				return
			}
		}
	}
	e.insertRuneRaw(r)
}

func (e *multilineEditor) findWrapSplit(prompt string) int {
	line := e.lines[e.row]
	promptCells := visibleCells(prompt)
	for i := len(line) - 1; i >= 0; i-- {
		if line[i] == ' ' {
			if promptCells+runesCells(line[:i]) <= e.width {
				return i
			}
		}
	}
	return -1
}

func (e *multilineEditor) insertNewline() {
	line := e.lines[e.row]
	next := append([]rune(nil), line[e.col:]...)
	e.lines[e.row] = append([]rune(nil), line[:e.col]...)
	e.lines = append(e.lines[:e.row+1], append([][]rune{next}, e.lines[e.row+1:]...)...)
	e.row++
	e.col = 0
}

func (e *multilineEditor) backspace() {
	line := e.lines[e.row]
	if newLine, newPos, ok := llm.BackspaceOverAtomicReplToken(line, e.col); ok {
		e.lines[e.row] = newLine
		e.col = newPos
		return
	}
	if e.col > 0 {
		e.lines[e.row] = append(line[:e.col-1], line[e.col:]...)
		e.col--
		return
	}
	if e.row == 0 {
		return
	}
	prevLen := len(e.lines[e.row-1])
	e.lines[e.row-1] = append(e.lines[e.row-1], e.lines[e.row]...)
	e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
	e.row--
	e.col = prevLen
}

func (e *multilineEditor) deleteForward() {
	line := e.lines[e.row]
	if newLine, newPos, ok := llm.DeleteForwardOverAtomicReplToken(line, e.col); ok {
		e.lines[e.row] = newLine
		e.col = newPos
		return
	}
	if e.col < len(line) {
		e.lines[e.row] = append(line[:e.col], line[e.col+1:]...)
		return
	}
	if e.row+1 < len(e.lines) {
		e.lines[e.row] = append(e.lines[e.row], e.lines[e.row+1]...)
		e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
	}
}

func (e *multilineEditor) left() {
	if newPos := llm.JumpLeftOverAtomicReplToken(e.lines[e.row], e.col); newPos >= 0 {
		e.col = newPos
		return
	}
	if e.col > 0 {
		e.col--
		return
	}
	if e.row > 0 {
		e.row--
		e.col = len(e.lines[e.row])
	}
}

func (e *multilineEditor) right() {
	if e.cursorAtBufferEnd() && len(e.suggestSuffix) > 0 {
		if e.completeAtMention() {
			return
		}
		e.acceptSuggest(false)
		return
	}
	if newPos := llm.JumpRightOverAtomicReplToken(e.lines[e.row], e.col); newPos >= 0 {
		e.col = newPos
		return
	}
	if e.col < len(e.lines[e.row]) {
		e.col++
		return
	}
	if e.row+1 < len(e.lines) {
		e.row++
		e.col = 0
	}
}

func (e *multilineEditor) up() {
	if e.row == 0 {
		e.loadHistoryPrev()
		return
	}
	want := e.col
	e.row--
	if want > len(e.lines[e.row]) {
		want = len(e.lines[e.row])
	}
	e.col = want
}

func (e *multilineEditor) down() {
	if e.row+1 == len(e.lines) {
		e.loadHistoryNext()
		return
	}
	want := e.col
	e.row++
	if want > len(e.lines[e.row]) {
		want = len(e.lines[e.row])
	}
	e.col = want
}

func (e *multilineEditor) loadHistoryPrev() {
	if s, ok := e.history.prev(e.string()); ok {
		e.setString(s, 0)
		e.clearSuggest()
	}
}

func (e *multilineEditor) loadHistoryNext() {
	if s, ok := e.history.next(); ok {
		e.setString(s, len(strings.Split(s, "\n"))-1)
		e.clearSuggest()
	}
}

func (e *multilineEditor) complete() bool {
	line := e.lines[e.row]
	candidates, _ := replcomplete.ReplCompleteDo(e.host.CompleteEnv, line, e.col)
	if len(candidates) == 0 {
		return false
	}
	insert := candidates[0]
	if len(candidates) > 1 {
		insert = commonRunePrefix(candidates)
	}
	if len(insert) == 0 {
		return false
	}
	for _, r := range insert {
		e.insertRuneRaw(r)
	}
	e.recomputeSuggest()
	return true
}

func commonRunePrefix(candidates [][]rune) []rune {
	if len(candidates) == 0 {
		return nil
	}
	prefix := append([]rune(nil), candidates[0]...)
	for _, candidate := range candidates[1:] {
		n := 0
		for n < len(prefix) && n < len(candidate) && prefix[n] == candidate[n] {
			n++
		}
		prefix = prefix[:n]
		if len(prefix) == 0 {
			break
		}
	}
	return prefix
}

func (e *multilineEditor) setString(s string, row int) {
	parts := strings.Split(s, "\n")
	e.lines = make([][]rune, len(parts))
	for i, p := range parts {
		e.lines[i] = []rune(p)
	}
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	if row < 0 {
		row = 0
	}
	if row >= len(e.lines) {
		row = len(e.lines) - 1
	}
	e.row = row
	e.col = len(e.lines[e.row])
}

func (e *multilineEditor) empty() bool {
	return len(e.lines) == 1 && len(e.lines[0]) == 0
}

func (e *multilineEditor) string() string {
	parts := make([]string, len(e.lines))
	for i, line := range e.lines {
		parts[i] = string(line)
	}
	return strings.Join(parts, "\n")
}
