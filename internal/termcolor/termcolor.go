package termcolor

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

const (
	User      = "\033[96m"
	Assistant = "\033[92m"
	Tool      = "\033[90m"
	Thinking  = "\033[38;2;255;246;157m"
	White     = "\033[97m"
	Reset     = "\033[0m"
)

func UsageTokensLine(promptTokens, reasoningTokens, responseTokens, totalTokens int64, outputTPS, ttftSecs, promptTPS float64) string {
	return "token: " +
		User + strconv.FormatInt(promptTokens, 10) + Reset + "+" +
		Thinking + strconv.FormatInt(reasoningTokens, 10) + Reset + "+" +
		Assistant + strconv.FormatInt(responseTokens, 10) + Reset + "=" +
		White + strconv.FormatInt(totalTokens, 10) + Reset +
		fmt.Sprintf("\t%.1ft/s ttft:%.3fs pp:%.1ft/s", outputTPS, ttftSecs, promptTPS)
}

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
		if _, err := io.WriteString(w.W, Tool); err != nil {
			return err
		}
		if _, err := w.W.Write(line); err != nil {
			return err
		}
		_, err := io.WriteString(w.W, Reset)
		return err
	}
	_, err := w.W.Write(line)
	return err
}
