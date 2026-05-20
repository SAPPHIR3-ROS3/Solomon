package agentruntime

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"unicode"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/clipboard"
)

const softNewlineRune = '\u2063'
const bracketedPasteStart = "\x1b[200~"
const bracketedPasteEnd = "\x1b[201~"
const bracketedPasteEnable = "\x1b[?2004h"
const bracketedPasteDisable = "\x1b[?2004l"
const mouseReportDisable = "\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l"

type stdinReadCloser interface {
	io.Reader
	io.Closer
}

func trimMessageEdges(s string) string {
	rs := []rune(s)
	start := 0
	end := len(rs)
	for start < end && unicode.IsSpace(rs[start]) {
		start++
	}
	for end > start && unicode.IsSpace(rs[end-1]) {
		end--
	}
	return string(rs[start:end])
}

func parseMultilineControlRunes(s string) (clean string, softBreak bool) {
	if s == "" {
		return s, false
	}
	softBreak = strings.ContainsRune(s, softNewlineRune)
	if !softBreak {
		return s, false
	}
	s = strings.ReplaceAll(s, string(softNewlineRune), "")
	return s, softBreak
}

func NewMultilineStdin(inner stdinReadCloser) io.ReadCloser {
	return &seqTranslator{inner: inner}
}

var replImagePaste func() (tag string, ok bool)

func SetReplImagePaste(fn func() (string, bool)) {
	replImagePaste = fn
}

type seqTranslator struct {
	inner               stdinReadCloser
	prefix              []byte
	outHold             []byte
	readBuf             []byte
	inBracketedPaste    bool
	bracketedPasteEmpty bool
	mouseAcc            []byte
}

func isPrefixOf(full string, part []byte) bool {
	if len(part) > len(full) {
		return false
	}
	return string(part) == full[:len(part)]
}

func isMouseAccChar(c byte) bool {
	switch {
	case c >= '0' && c <= '9':
		return true
	case c == ';', c == '<', c == '[', c == '?':
		return true
	default:
		return false
	}
}

func isMouseReportBody(body string) bool {
	if !strings.Contains(body, ";") {
		return false
	}
	for i := 0; i < len(body); i++ {
		c := body[i]
		if (c < '0' || c > '9') && c != ';' {
			return false
		}
	}
	return true
}

func isMouseReportBytes(b []byte) bool {
	if len(b) < 5 {
		return false
	}
	s := string(b)
	switch {
	case strings.HasPrefix(s, "\x1b[<"):
		s = s[3:]
	case len(s) > 0 && s[0] == '[':
		s = s[1:]
	}
	if len(s) < 4 {
		return false
	}
	last := s[len(s)-1]
	if last != 'M' && last != 'm' {
		return false
	}
	return isMouseReportBody(s[:len(s)-1])
}

func isTerminalModeReportBytes(b []byte) bool {
	s := string(b)
	switch {
	case strings.HasPrefix(s, "\x1b[?"):
		s = s[3:]
	case strings.HasPrefix(s, "[?"):
		s = s[2:]
	default:
		return false
	}
	if len(s) < 2 {
		return false
	}
	last := s[len(s)-1]
	if last != 'l' && last != 'h' {
		return false
	}
	for i := 0; i < len(s)-1; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func (s *seqTranslator) mouseFilterStep(c byte) (emit []byte, consumed bool) {
	if len(s.mouseAcc) == 0 {
		if c == 0x1b || c == '[' || (c >= '0' && c <= '9') {
			s.mouseAcc = append(s.mouseAcc, c)
			return nil, true
		}
		return nil, false
	}
	if c == 'M' || c == 'm' {
		s.mouseAcc = append(s.mouseAcc, c)
		if isMouseReportBytes(s.mouseAcc) {
			s.mouseAcc = nil
			return nil, true
		}
		out := append([]byte(nil), s.mouseAcc...)
		s.mouseAcc = nil
		return out, true
	}
	if c == 'l' || c == 'h' {
		s.mouseAcc = append(s.mouseAcc, c)
		if isTerminalModeReportBytes(s.mouseAcc) {
			s.mouseAcc = nil
			return nil, true
		}
		out := append([]byte(nil), s.mouseAcc...)
		s.mouseAcc = nil
		return out, true
	}
	if isMouseAccChar(c) {
		s.mouseAcc = append(s.mouseAcc, c)
		if len(s.mouseAcc) > 64 {
			out := append([]byte(nil), s.mouseAcc...)
			s.mouseAcc = nil
			return out, false
		}
		return nil, true
	}
	out := append([]byte(nil), s.mouseAcc...)
	s.mouseAcc = nil
	return out, false
}

func (s *seqTranslator) applyMouseFilterAtHead() bool {
	if len(s.prefix) == 0 {
		return false
	}
	emit, consumed := s.mouseFilterStep(s.prefix[0])
	if !consumed {
		return false
	}
	s.prefix = s.prefix[1:]
	if len(emit) > 0 {
		s.prefix = append(emit, s.prefix...)
	}
	return true
}

func (s *seqTranslator) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(s.outHold) > 0 {
		n := copy(p, s.outHold)
		s.outHold = s.outHold[n:]
		return n, nil
	}
	spins := 0
	for len(s.outHold) == 0 {
		spins++
		if spins > 4096 {
			s.outHold = append([]byte(nil), s.prefix...)
			s.prefix = nil
			break
		}
		if s.readBuf == nil {
			s.readBuf = make([]byte, 512)
		}
		nr, err := s.inner.Read(s.readBuf)
		if nr > 0 {
			s.prefix = append(s.prefix, s.readBuf[:nr]...)
		}
		out := s.flushPrefix()
		if len(out) > 0 {
			s.outHold = out
			break
		}
		if len(s.prefix) == 0 {
			return 0, err
		}
		if nr == 0 && err != nil {
			s.outHold = append([]byte(nil), s.prefix...)
			s.prefix = nil
			break
		}
		if len(s.prefix) > 8192 {
			s.outHold = append([]byte(nil), s.prefix...)
			s.prefix = nil
			break
		}
	}
	if len(s.outHold) == 0 {
		return 0, nil
	}
	n := copy(p, s.outHold)
	s.outHold = s.outHold[n:]
	return n, nil
}

func (s *seqTranslator) finishBracketedPaste(b *bytes.Buffer) {
	s.inBracketedPaste = false
	empty := s.bracketedPasteEmpty
	s.bracketedPasteEmpty = false
	if !empty {
		return
	}
	if replImagePaste != nil {
		if tag, ok := replImagePaste(); ok && tag != "" {
			b.WriteString(tag)
			return
		}
	}
	if clipboard.HasImage() {
		b.WriteByte(replImagePasteKey)
	}
}

func (s *seqTranslator) flushPrefix() []byte {
	var b bytes.Buffer
	for len(s.prefix) > 0 {
		if s.inBracketedPaste {
			if isPrefixOf(bracketedPasteEnd, s.prefix) && len(s.prefix) < len(bracketedPasteEnd) {
				return b.Bytes()
			}
			if bytes.HasPrefix(s.prefix, []byte(bracketedPasteEnd)) {
				s.prefix = s.prefix[len(bracketedPasteEnd):]
				s.finishBracketedPaste(&b)
				continue
			}
			if s.prefix[0] == '\r' {
				if len(s.prefix) >= 2 && s.prefix[1] == '\n' {
					s.prefix = s.prefix[2:]
				} else {
					s.prefix = s.prefix[1:]
				}
				s.bracketedPasteEmpty = false
				b.WriteString(string(softNewlineRune))
				b.WriteByte('\r')
				continue
			}
			if s.prefix[0] == '\n' {
				s.prefix = s.prefix[1:]
				s.bracketedPasteEmpty = false
				b.WriteString(string(softNewlineRune))
				b.WriteByte('\r')
				continue
			}
			s.bracketedPasteEmpty = false
			b.WriteByte(s.prefix[0])
			s.prefix = s.prefix[1:]
			continue
		}
		if isPrefixOf(bracketedPasteStart, s.prefix) && len(s.prefix) < len(bracketedPasteStart) {
			return b.Bytes()
		}
		if bytes.HasPrefix(s.prefix, []byte(bracketedPasteStart)) {
			s.prefix = s.prefix[len(bracketedPasteStart):]
			s.inBracketedPaste = true
			s.bracketedPasteEmpty = true
			continue
		}
		if s.applyMouseFilterAtHead() {
			continue
		}
		if s.prefix[0] == '\n' {
			s.prefix = s.prefix[1:]
			b.WriteString(string(softNewlineRune))
			b.WriteByte('\r')
			continue
		}
		if s.prefix[0] == '\r' && len(s.prefix) >= 2 && s.prefix[1] == '\n' {
			s.prefix = s.prefix[2:]
			b.WriteString(string(softNewlineRune))
			b.WriteByte('\r')
			continue
		}
		if s.prefix[0] != 0x1b {
			b.WriteByte(s.prefix[0])
			s.prefix = s.prefix[1:]
			continue
		}
		if len(s.prefix) == 1 {
			return b.Bytes()
		}
		if s.prefix[1] == '\n' || s.prefix[1] == '\r' {
			b.WriteString(string(softNewlineRune))
			b.WriteByte('\r')
			s.prefix = s.prefix[2:]
			continue
		}
		if s.prefix[1] == 'M' && len(s.prefix) >= 4 {
			s.prefix = s.prefix[4:]
			continue
		}
		if s.prefix[1] != '[' {
			b.WriteByte(s.prefix[0])
			s.prefix = s.prefix[1:]
			continue
		}
		end := csiFinalIndex(s.prefix)
		if end < 0 {
			return b.Bytes()
		}
		seq := string(s.prefix[:end])
		if isSoftNewlineCSI(seq) {
			b.WriteString(string(softNewlineRune))
			b.WriteByte('\r')
		} else if !isMouseCSI(seq) && !isTerminalModeCSI(seq) {
			b.Write(s.prefix[:end])
		}
		s.prefix = s.prefix[end:]
	}
	return b.Bytes()
}

func csiFinalIndex(buf []byte) int {
	if len(buf) < 3 || buf[0] != 0x1b || buf[1] != '[' {
		return -1
	}
	for i := 2; i < len(buf); i++ {
		c := buf[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '~' || c == 'u' {
			return i + 1
		}
	}
	return -1
}

func isMouseCSI(seq string) bool {
	if !strings.HasPrefix(seq, "\x1b[<") || len(seq) == 0 {
		return false
	}
	last := seq[len(seq)-1]
	return last == 'M' || last == 'm'
}

func isTerminalModeCSI(seq string) bool {
	return isTerminalModeReportBytes([]byte(seq))
}

func isSoftNewlineCSI(seq string) bool {
	return strings.Contains(seq, "13;2") || strings.Contains(seq, "13;5") ||
		strings.Contains(seq, "27;5;13") || strings.Contains(seq, ";5;13") ||
		strings.Contains(seq, "13;3") || strings.Contains(seq, "27;2") ||
		strings.Contains(seq, "13u")
}

func (s *seqTranslator) Close() error {
	if s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func PlatformStdin() stdinReadCloser {
	return platformStdin()
}

func writeTerminalModeSequences(seq string) {
	if seq == "" {
		return
	}
	if runtime.GOOS == "windows" {
		_, _ = os.Stdout.Write([]byte(seq))
		return
	}
	_, _ = io.WriteString(os.Stdout, seq)
}

func enableReplInputModes(w io.Writer) func() {
	if w == nil {
		return func() {}
	}
	restoreConsole := prepareConsoleInput()
	writeTerminalModeSequences(mouseReportDisable + bracketedPasteEnable)
	return func() {
		writeTerminalModeSequences(bracketedPasteDisable)
		restoreConsole()
	}
}
