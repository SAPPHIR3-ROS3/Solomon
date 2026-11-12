package main

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

type inputChunk struct {
	data []byte
	err  error
}
type terminalInput struct {
	onMouse        func(tea.MouseMsg)
	file           *os.File
	once           sync.Once
	chunks         chan inputChunk
	pending, ready []byte
	finished       bool
	pasted         bool
}

func newTerminalInput(file *os.File, click func(tea.MouseMsg)) *terminalInput {
	return &terminalInput{file: file, chunks: make(chan inputChunk, 8), onMouse: click}
}
func (r *terminalInput) start() {
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := r.file.Read(buffer)
			chunk := append([]byte(nil), buffer[:n]...)
			r.chunks <- inputChunk{chunk, err}
			if err != nil {
				return
			}
		}
	}()
}
func (r *terminalInput) Read(out []byte) (int, error) {
	r.once.Do(r.start)
	for len(r.ready) == 0 {
		r.consume()
		if len(r.ready) > 0 {
			break
		}
		if r.finished {
			r.pending = nil
			return 0, io.EOF
		}
		if len(r.pending) == 0 || bytes.HasPrefix(r.pending, []byte("\x1b[")) {
			chunk := <-r.chunks
			r.pending = append(r.pending, chunk.data...)
			r.finished = chunk.err != nil
		} else {
			timer := time.NewTimer(60 * time.Millisecond)
			select {
			case chunk := <-r.chunks:
				r.pending = append(r.pending, chunk.data...)
				r.finished = chunk.err != nil
			case <-timer.C:
				if bytes.Equal(r.pending, []byte{27}) {
					r.ready = append(r.ready, 27)
				}
				r.pending = nil
			}
			timer.Stop()
		}
	}
	n := copy(out, r.ready)
	r.ready = r.ready[n:]
	return n, nil
}
func (r *terminalInput) consume() {
	for len(r.pending) > 0 {
		if r.pasted {
			end := bytes.Index(r.pending, []byte("\x1b[201~"))
			if end < 0 {
				if len(r.pending) > 6 {
					n := len(r.pending) - 6
					r.ready = append(r.ready, r.pending[:n]...)
					r.pending = r.pending[n:]
				}
				return
			}
			r.ready = append(r.ready, r.pending[:end+6]...)
			r.pending = r.pending[end+6:]
			r.pasted = false
			continue
		}
		if r.pending[0] != 27 {
			r.ready = append(r.ready, r.pending[0])
			r.pending = r.pending[1:]
			continue
		}
		if len(r.pending) < 2 {
			return
		}
		if r.pending[1] != '[' {
			r.ready = append(r.ready, r.pending[:2]...)
			r.pending = r.pending[2:]
			continue
		}
		if len(r.pending) < 3 {
			return
		}
		if r.pending[2] == 'M' {
			if len(r.pending) < 6 {
				return
			}
			code := int(r.pending[3]) - 32
			if code&64 != 0 {
				r.wheel(code & 1)
			} else {
				r.click(code, int(r.pending[4])-33, int(r.pending[5])-33, code&3 == 3)
			}
			r.pending = r.pending[6:]
			continue
		}
		end := 2
		for end < len(r.pending) && (r.pending[end] < 0x40 || r.pending[end] > 0x7e) {
			end++
		}
		if end == len(r.pending) {
			return
		}
		seq := string(r.pending[:end+1])
		r.pending = r.pending[end+1:]
		if strings.HasPrefix(seq, "\x1b[<") {
			fields := strings.Split(seq[3:len(seq)-1], ";")
			if len(fields) == 3 {
				code, codeErr := strconv.Atoi(fields[0])
				x, xErr := strconv.Atoi(fields[1])
				y, yErr := strconv.Atoi(fields[2])
				if codeErr == nil && xErr == nil && yErr == nil && x > 0 && y > 0 && x <= 10000 && y <= 10000 {
					if code&64 != 0 {
						if seq[len(seq)-1] == 'M' {
							r.wheel(code & 1)
						}
					} else {
						r.click(code, x-1, y-1, seq[len(seq)-1] == 'm')
					}
				}
			}
			continue
		}
		switch seq {
		case "\x1b[200~":
			r.pasted = true
			r.ready = append(r.ready, []byte(seq)...)
		case "\x1b[13;2u", "\x1b[27;2;13~":
			r.ready = append(r.ready, []byte("\x1b\r")...)
		default:
			r.ready = append(r.ready, []byte(seq)...)
		}
	}
}
func (r *terminalInput) wheel(direction int) {
	key := "\x1b[31~"
	if direction != 0 {
		key = "\x1b[32~"
	}
	r.ready = append(r.ready, []byte(key)...)
}

func runPreview(m model) (tea.Model, error) {
	var original *term.State
	if term.IsTerminal(os.Stdin.Fd()) {
		state, err := term.MakeRaw(os.Stdin.Fd())
		if err != nil {
			return m, err
		}
		original = state
		defer term.Restore(os.Stdin.Fd(), original)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return m, err
	}
	defer reader.Close()
	defer writer.Close()
	go func() { _, _ = io.Copy(writer, newTerminalInput(os.Stdin, m.mouseEvents.Push)); _ = writer.Close() }()
	options := []tea.ProgramOption{tea.WithInput(reader), tea.WithAltScreen()}
	if m.liveCfg.Mouse {
		options = append(options, tea.WithMouseCellMotion())
	}
	return tea.NewProgram(m, options...).Run()
}

func (r *terminalInput) click(code, x, y int, release bool) {
	if r.onMouse == nil || x < 0 || y < 0 || code&32 != 0 || release {
		return
	}
	button := tea.MouseButtonNone
	switch code & 3 {
	case 0:
		button = tea.MouseButtonLeft
	case 1:
		button = tea.MouseButtonMiddle
	case 2:
		button = tea.MouseButtonRight
	}
	action := tea.MouseActionPress
	if release {
		action = tea.MouseActionRelease
	}
	r.onMouse(tea.MouseMsg{X: x, Y: y, Button: button, Action: action, Shift: code&4 != 0, Alt: code&8 != 0, Ctrl: code&16 != 0})
	r.ready = append(r.ready, 0)
}

type mouseQueue struct {
	mu     sync.Mutex
	events []tea.MouseMsg
}

func (q *mouseQueue) Push(event tea.MouseMsg) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.events = append(q.events, event)
}
func (q *mouseQueue) Pop() (tea.MouseMsg, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.events) == 0 {
		return tea.MouseMsg{}, false
	}
	event := q.events[0]
	q.events = q.events[1:]
	return event, true
}
