package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"

	readline "github.com/chzyer/readline"
	"golang.org/x/term"
)

type multilineEditor struct {
	loop          *Loop
	history       *inputHistory
	lines         [][]rune
	row           int
	col           int
	width         int
	rendered      int
	cursorLine    int
	suggestSuffix []rune
	atMatches     []atmention.Entry
	atSelected    int
	atCtx         atmention.AtContext
	out              io.Writer
	wrapDisabled     bool
	wrapClearPrevRow bool
}

func readMultilineInput(loop *Loop, history *inputHistory) (string, error) {
	restoreRaw, err := multiline.EnterRawStdin()
	if err != nil {
		return "", err
	}
	defer restoreRaw()
	fd := int(os.Stdin.Fd())

	width := 80
	if loop.RL != nil && loop.RL.Config != nil && loop.RL.Config.FuncGetWidth != nil {
		if w := loop.RL.Config.FuncGetWidth(); w > 8 {
			width = w
		}
	} else if w, _, err := term.GetSize(fd); err == nil && w > 8 {
		width = w
	}
	e := &multilineEditor{
		loop:    loop,
		history: history,
		lines:   [][]rune{{}},
		width:   width,
		out:     multiline.EditorStdout(loop.RL.Stdout()),
	}
	defer e.finish()
	e.refresh()
	reader := bufio.NewReader(os.Stdin)
	for {
		key, err := readEditorKey(reader)
		if err != nil {
			if errors.Is(err, io.EOF) && e.empty() {
				return "", io.EOF
			}
			return "", err
		}
		if done, line, err := e.handle(key); done || err != nil {
			return line, err
		}
	}
}

type editorKey struct {
	r     rune
	seq   string
	text  string
	paste bool
}

func readEditorKey(r *bufio.Reader) (editorKey, error) {
	ch, _, err := r.ReadRune()
	if err != nil {
		return editorKey{}, err
	}
	if ch != readline.CharEsc {
		return editorKey{r: ch}, nil
	}
	var b strings.Builder
	b.WriteRune(ch)
	for r.Buffered() > 0 || stdinReady(20*time.Millisecond) {
		next, _, err := r.ReadRune()
		if err != nil {
			return editorKey{}, err
		}
		b.WriteRune(next)
		s := b.String()
		if strings.HasPrefix(s, "\x1b[200~") {
			return readBracketedPaste(r)
		}
		if isCompleteEscape(s) {
			return editorKey{seq: s}, nil
		}
	}
	s := b.String()
	if strings.HasPrefix(s, "\x1b[200~") {
		return readBracketedPaste(r)
	}
	return editorKey{seq: s}, nil
}

func isCompleteEscape(s string) bool {
	if len(s) < 2 {
		return false
	}
	last := s[len(s)-1]
	return (last >= 'A' && last <= 'Z') || (last >= 'a' && last <= 'z') || last == '~' || last == 'u'
}

func readBracketedPaste(r *bufio.Reader) (editorKey, error) {
	var b strings.Builder
	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			return editorKey{}, err
		}
		b.WriteRune(ch)
		s := b.String()
		if strings.HasSuffix(s, "\x1b[201~") {
			return editorKey{text: strings.TrimSuffix(s, "\x1b[201~"), paste: true}, nil
		}
	}
}

func (e *multilineEditor) handle(key editorKey) (bool, string, error) {
	resetHistory := false
	switch {
	case key.paste:
		e.insertPaste(key.text)
		resetHistory = true
		e.clearSuggest()
	case key.seq != "":
		return e.handleSeq(key.seq)
	case key.r == readline.CharInterrupt:
		return true, "", readline.ErrInterrupt
	case key.r == 4:
		if e.empty() {
			return true, "", io.EOF
		}
	case key.r == '\r' || key.r == '\n':
		if e.completeAtMention() {
			e.recomputeAtPicker()
			e.refresh()
			return false, "", nil
		}
		return true, e.string(), nil
	case key.r == readline.CharBackspace || key.r == readline.CharCtrlH:
		e.backspace()
		resetHistory = true
	case key.r == readline.CharDelete:
		e.deleteForward()
		resetHistory = true
	case key.r == readline.CharTab:
		if e.completeAtMention() {
			resetHistory = true
		} else {
			resetHistory = e.complete()
		}
	case key.r == readline.CharLineStart:
		e.col = 0
	case key.r == readline.CharLineEnd:
		if e.cursorAtBufferEnd() && len(e.suggestSuffix) > 0 {
			e.acceptSuggest(false)
		} else {
			e.col = len(e.lines[e.row])
		}
	case key.r == readline.CharBackward:
		e.left()
	case key.r == readline.CharForward:
		e.right()
	case key.r == readline.CharPrev:
		e.up()
	case key.r == readline.CharNext:
		e.down()
	default:
		if key.r >= 32 && key.r != utf8.RuneError {
			e.insertRune(key.r)
			resetHistory = true
		}
	}
	if resetHistory {
		e.history.resetNav()
	}
	e.recomputeSuggest()
	e.recomputeAtPicker()
	if e.wrapClearPrevRow {
		e.wrapClearPrevRow = false
		fmt.Fprint(e.out, "\x1b[1A\x1b[2K\x1b[1B\r\x1b[2K")
		e.cursorLine = 0
		e.rendered = 0
	}
	e.refresh()
	return false, "", nil
}

func (e *multilineEditor) handleSeq(seq string) (bool, string, error) {
	resetHistory := false
	clearSuggest := false
	switch seq {
	case "\x1b[A", "\x1bOA":
		if e.atPickerActive() {
			e.atPickerUp()
		} else {
			e.up()
		}
		clearSuggest = true
	case "\x1b[B", "\x1bOB":
		if e.atPickerActive() {
			e.atPickerDown()
		} else {
			e.down()
		}
		clearSuggest = true
	case "\x1b[D", "\x1bOD":
		e.left()
		clearSuggest = true
	case "\x1b[C", "\x1bOC":
		e.right()
	case "\x1b[1;3C", "\x1b[1;5C":
		if e.cursorAtBufferEnd() && len(e.suggestSuffix) > 0 {
			e.acceptSuggest(true)
		} else {
			e.right()
		}
	case "\x1b[H", "\x1bOH":
		e.col = 0
		clearSuggest = true
	case "\x1b[F", "\x1bOF":
		if e.cursorAtBufferEnd() && len(e.suggestSuffix) > 0 {
			e.acceptSuggest(false)
		} else {
			e.col = len(e.lines[e.row])
		}
	case "\x1b[3~":
		e.deleteForward()
		resetHistory = true
	case "\x1b\r", "\x1b\n", "\x1b[13;2u", "\x1b[13;5u", "\x1b[27;5;13~":
		e.insertNewline()
		resetHistory = true
	default:
		return false, "", nil
	}
	if resetHistory {
		e.history.resetNav()
	}
	if clearSuggest {
		e.clearSuggest()
	} else if !resetHistory {
		e.recomputeSuggest()
	}
	e.recomputeAtPicker()
	e.refresh()
	return false, "", nil
}

func (e *multilineEditor) insertPaste(s string) {
	e.wrapDisabled = true
	if s == "" {
		if e.loop.ClipboardPasteForStdin != nil {
			if tag, ok := e.loop.ClipboardPasteForStdin(); ok {
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
				e.col = splitAt + 1 // after the space
				e.insertNewline()
				// Position cursor at the end of the overflow word on the
				// new line so the typed character is appended, not prepended.
				e.col = len(e.lines[e.row])
				e.insertRuneRaw(r)
				e.suggestSuffix = nil
				return
			}
		}
	}
	e.insertRuneRaw(r)
}

// findWrapSplit cerca nella riga corrente l'ultimo spazio (da destra)
// tale che la parte di riga prima dello spazio + il prompt stia entro e.width.
// Restituisce l'indice dello spazio, o -1 se non trovato.
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
	candidates, _ := replcomplete.ReplCompleteDo(e.loop.CompleteEnv, line, e.col)
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
