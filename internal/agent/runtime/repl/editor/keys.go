package editor

import (
	"bufio"
	"strings"
	"time"
	"unicode/utf8"

	readline "github.com/chzyer/readline"
)

type editorKey struct {
	r     rune
	seq   string
	text  string
	paste bool
}

func readEditorKey(r *bufio.Reader, interrupt <-chan struct{}) (editorKey, error) {
	for {
		if interrupt != nil {
			select {
			case <-interrupt:
				return editorKey{}, ErrInputInterrupted
			default:
			}
		}
		if !stdinReady(50 * time.Millisecond) {
			continue
		}
		ch, err := readInputRune(r)
		if err != nil {
			return editorKey{}, err
		}
		if ch != readline.CharEsc {
			return editorKey{r: ch}, nil
		}
		return readEditorKeyEscape(r, ch)
	}
}

func readEditorKeyEscape(r *bufio.Reader, first rune) (editorKey, error) {
	var b strings.Builder
	b.WriteRune(first)
	for r.Buffered() > 0 || stdinReady(20*time.Millisecond) {
		next, err := readInputRune(r)
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
		ch, err := readInputRune(r)
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

func readInputRune(r *bufio.Reader) (rune, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if b < utf8.RuneSelf {
		return rune(b), nil
	}
	buf := []byte{b}
	for len(buf) < utf8.UTFMax && !utf8.FullRune(buf) {
		if r.Buffered() == 0 && !stdinReady(20*time.Millisecond) {
			break
		}
		next, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		buf = append(buf, next)
	}
	ch, _ := utf8.DecodeRune(buf)
	return ch, nil
}
