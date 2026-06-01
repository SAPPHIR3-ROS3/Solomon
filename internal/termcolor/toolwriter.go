package termcolor

import (
	"bytes"
	"io"
	"strings"
)

type ToolLineWriter struct {
	W io.Writer
	b []byte
}

func NewToolLineWriter(w io.Writer) *ToolLineWriter {
	return &ToolLineWriter{W: w}
}

func (w *ToolLineWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	for {
		i := bytes.IndexByte(w.b, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := w.b[:i+1]
		w.b = w.b[i+1:]
		if err := w.writeLine(line); err != nil {
			return len(p), err
		}
	}
}

func (w *ToolLineWriter) Flush() error {
	if len(w.b) == 0 {
		return nil
	}
	line := w.b
	w.b = nil
	return w.writeLine(line)
}

func (w *ToolLineWriter) writeLine(line []byte) error {
	trim := bytes.TrimSpace(line)
	if bytes.HasPrefix(trim, []byte("Tool:")) {
		_, err := io.WriteString(w.W, WrapTool(string(trim))+"\n")
		return err
	}
	_, err := w.W.Write(line)
	return err
}

type ErrorLineWriter struct {
	W         io.Writer
	prefix    []byte
	lineStart bool
	lineError bool
}

func NewErrorLineWriter(w io.Writer) *ErrorLineWriter {
	return &ErrorLineWriter{W: w, lineStart: true}
}

func (w *ErrorLineWriter) Write(p []byte) (int, error) {
	marker := []byte("[error]")
	for i := 0; i < len(p); {
		if w.lineError {
			j := bytes.IndexByte(p[i:], '\n')
			if j < 0 {
				if _, err := io.WriteString(w.W, WrapRed(string(p[i:]))); err != nil {
					return len(p), err
				}
				return len(p), nil
			}
			j += i + 1
			if _, err := io.WriteString(w.W, WrapRed(string(p[i:j]))); err != nil {
				return len(p), err
			}
			w.lineError = false
			w.lineStart = true
			i = j
			continue
		}
		if !w.lineStart {
			j := bytes.IndexByte(p[i:], '\n')
			if j < 0 {
				if _, err := w.W.Write(p[i:]); err != nil {
					return len(p), err
				}
				return len(p), nil
			}
			j += i + 1
			if _, err := w.W.Write(p[i:j]); err != nil {
				return len(p), err
			}
			w.lineStart = true
			i = j
			continue
		}
		w.prefix = append(w.prefix, p[i])
		i++
		trim := bytes.TrimLeft(w.prefix, " \t\r")
		if len(trim) == 0 {
			continue
		}
		if trim[0] == '\n' {
			if _, err := w.W.Write(w.prefix); err != nil {
				return len(p), err
			}
			w.prefix = nil
			continue
		}
		if len(trim) < len(marker) && bytes.Equal(trim, marker[:len(trim)]) {
			continue
		}
		if bytes.HasPrefix(trim, marker) {
			if _, err := io.WriteString(w.W, WrapRed(string(w.prefix))); err != nil {
				return len(p), err
			}
			w.prefix = nil
			w.lineStart = false
			w.lineError = true
			continue
		}
		if _, err := w.W.Write(w.prefix); err != nil {
			return len(p), err
		}
		w.prefix = nil
		w.lineStart = false
	}
	return len(p), nil
}

func (w *ErrorLineWriter) Flush() error {
	if len(w.prefix) == 0 {
		return nil
	}
	line := w.prefix
	w.prefix = nil
	_, err := w.W.Write(line)
	return err
}

func ColorizeErrorLines(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		line := s
		suffix := ""
		if i >= 0 {
			line = s[:i]
			suffix = "\n"
			s = s[i+1:]
		} else {
			s = ""
		}
		if strings.HasPrefix(strings.TrimSpace(line), "[error]") {
			b.WriteString(WrapRed(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString(suffix)
	}
	return b.String()
}
