package listener

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/btw/input"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"golang.org/x/term"
)

type Listener struct {
	onCollectStart func() (line string, ok bool)
	onSubmit       func(question string)
	onDiscard      func()
	onStop         func()
	done           chan struct{}
	wg             sync.WaitGroup
}

func New(onCollectStart func() (string, bool), onSubmit func(string), onDiscard func(), onStop func()) *Listener {
	return &Listener{
		onCollectStart: onCollectStart,
		onSubmit:       onSubmit,
		onDiscard:      onDiscard,
		onStop:         onStop,
		done:           make(chan struct{}),
	}
}

func (l *Listener) Start() {
	if l == nil || l.onSubmit == nil {
		return
	}
	l.wg.Add(1)
	go l.run()
}

func (l *Listener) Stop() {
	if l == nil {
		return
	}
	close(l.done)
	l.wg.Wait()
}

func (l *Listener) run() {
	defer l.wg.Done()
	in, err := input.OpenTerminal()
	if err != nil {
		return
	}
	if in != os.Stdin {
		defer in.Close()
	}
	fd := int(in.Fd())
	multiline.FlushStdin()
	restore, err := multiline.EnterCbreakFD(fd)
	if err != nil {
		return
	}
	defer func() {
		if restore != nil {
			restore()
		}
		multiline.EnsureCookedFD(fd)
		multiline.EnsureCookedTTY()
	}()
	rd := bufio.NewReader(in)
	for {
		select {
		case <-l.done:
			return
		default:
		}
		if !ready(fd, 50*time.Millisecond) {
			continue
		}
		r, _, err := rd.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errno, ok := err.(syscall.Errno); ok && (errno == syscall.EINTR || errno == syscall.EAGAIN) {
				continue
			}
			return
		}
		if r != '/' {
			continue
		}
		restore()
		restore = nil
		multiline.EnsureCookedFD(fd)
		multiline.EnsureCookedTTY()
		if !l.collectWithEditor() && l.onDiscard != nil {
			l.onDiscard()
		}
		restore, err = multiline.EnterCbreakFD(fd)
		if err != nil {
			return
		}
		rd = bufio.NewReader(in)
	}
}

func (l *Listener) collectWithEditor() bool {
	if l.onCollectStart == nil {
		return false
	}
	line, ok := l.onCollectStart()
	if !ok {
		return false
	}
	return l.handleLine(line)
}

func (l *Listener) handleLine(raw string) bool {
	q, ok := ParseSubmit(raw)
	if !ok {
		return false
	}
	l.onSubmit(q)
	return true
}

func ParseSubmit(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "/btw") {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(raw, "/btw"))
	if rest == "" {
		return "", false
	}
	return rest, true
}

func Available() bool {
	if f, err := input.OpenTerminal(); err == nil {
		if f != os.Stdin {
			_ = f.Close()
		}
		return true
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}
