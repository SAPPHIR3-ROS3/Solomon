package editor

import (
	"bufio"
	"errors"
	"io"
	"os"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"

	readline "github.com/chzyer/readline"
	"golang.org/x/term"
)

var ErrInputInterrupted = errors.New("repl: input interrupted")

type multilineEditor struct {
	host            Host
	history         *History
	lines           [][]rune
	row             int
	col             int
	width           int
	height          int
	viewportTop     int
	rendered        int
	cursorLine      int
	suggestSuffix   []rune
	atMatches       []atmention.Entry
	atSelected      int
	atCtx           atmention.AtContext
	atCycleActive   bool
	atCycleStart    int
	atCycleMatches  []atmention.Entry
	atCycleIndex    int
	compRow         int
	compWordStart   int
	compTypedPrefix []rune
	compCandidates  [][]rune
	compIndex       int
	compActive      bool
	out             io.Writer
}

func ReadMultiline(host Host, history *History) (string, error) {
	return ReadMultilineInitial(host, history, "")
}

func ReadMultilineInitial(host Host, history *History, initial string) (string, error) {
	restoreRaw, err := multiline.EnterRawStdin()
	if err != nil {
		return "", err
	}
	defer restoreRaw()
	fd := int(os.Stdin.Fd())

	width := 80
	height := 24
	if host.RL != nil && host.RL.Config != nil && host.RL.Config.FuncGetWidth != nil {
		if w := host.RL.Config.FuncGetWidth(); w > 8 {
			width = w
		}
	}
	if w, h, err := term.GetSize(fd); err == nil {
		if host.RL == nil || host.RL.Config == nil || host.RL.Config.FuncGetWidth == nil {
			if w > 8 {
				width = w
			}
		}
		if h > 4 {
			height = h
		}
	}
	out := host.Out
	if out == nil && host.RL != nil {
		out = multiline.EditorStdout(host.RL.Stdout())
	}
	e := &multilineEditor{
		host:    host,
		history: history,
		lines:   [][]rune{{}},
		width:   width,
		height:  height,
		out:     out,
	}
	if initial != "" {
		e.setString(initial, 0)
		e.recomputeSuggest()
		e.recomputeAtPicker()
	}
	defer e.finish()
	e.refresh()
	reader := bufio.NewReader(os.Stdin)
	interrupt := host.InputInterrupt
	for {
		key, err := readEditorKey(reader, interrupt)
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
		if e.completeAtMentionAccept() {
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
		if e.completeAtMentionTab() {
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
	case key.r == rune(multiline.PasteImageKey):
		e.insertPaste("")
		resetHistory = true
		e.clearSuggest()
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
