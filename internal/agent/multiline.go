package agent

import (
	"bytes"
	"io"
	"strings"
	"unicode"
)

const softNewlineRune = '\u2063'
const bracketedPasteStart = "\x1b[200~"
const bracketedPasteEnd = "\x1b[201~"
const bracketedPasteEnable = "\x1b[?2004h"
const bracketedPasteDisable = "\x1b[?2004l"

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

type seqTranslator struct {
	inner            stdinReadCloser
	prefix           []byte
	outHold          []byte
	readBuf          []byte
	inBracketedPaste bool
}

func isPrefixOf(full string, part []byte) bool {
	if len(part) > len(full) {
		return false
	}
	return string(part) == full[:len(part)]
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

func (s *seqTranslator) flushPrefix() []byte {
	var b bytes.Buffer
	for len(s.prefix) > 0 {
		if s.inBracketedPaste {
			if isPrefixOf(bracketedPasteEnd, s.prefix) && len(s.prefix) < len(bracketedPasteEnd) {
				return b.Bytes()
			}
			if bytes.HasPrefix(s.prefix, []byte(bracketedPasteEnd)) {
				s.prefix = s.prefix[len(bracketedPasteEnd):]
				s.inBracketedPaste = false
				continue
			}
			if s.prefix[0] == '\r' {
				if len(s.prefix) >= 2 && s.prefix[1] == '\n' {
					s.prefix = s.prefix[2:]
				} else {
					s.prefix = s.prefix[1:]
				}
				b.WriteString(string(softNewlineRune))
				b.WriteByte('\r')
				continue
			}
			if s.prefix[0] == '\n' {
				s.prefix = s.prefix[1:]
				b.WriteString(string(softNewlineRune))
				b.WriteByte('\r')
				continue
			}
			b.WriteByte(s.prefix[0])
			s.prefix = s.prefix[1:]
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
		if isPrefixOf(bracketedPasteStart, s.prefix) && len(s.prefix) < len(bracketedPasteStart) {
			return b.Bytes()
		}
		if bytes.HasPrefix(s.prefix, []byte(bracketedPasteStart)) {
			s.prefix = s.prefix[len(bracketedPasteStart):]
			s.inBracketedPaste = true
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
		} else {
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

func enableBracketedPasteMode(w io.Writer) func() {
	if w == nil {
		return func() {}
	}
	_, _ = io.WriteString(w, bracketedPasteEnable)
	return func() {
		_, _ = io.WriteString(w, bracketedPasteDisable)
	}
}
