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
	Tool      = "\033[38;2;255;246;157m"
	Thinking  = "\033[90m"
	White     = "\033[97m"
	Context   = "\033[94m"
	Reset     = "\033[0m"
)

func WrapUser(s string) string {
	return User + s + Reset
}

func WrapAssistant(s string) string {
	return Assistant + s + Reset
}

func WrapTool(s string) string {
	return Tool + s + Reset
}

func WrapThinking(s string) string {
	return Thinking + s + Reset
}

func WrapWhite(s string) string {
	return White + s + Reset
}

func WrapContext(s string) string {
	return Context + s + Reset
}

func formatContextPromptTok(n int64, estimated bool) string {
	s := strconv.FormatInt(n, 10)
	if estimated && n > 0 {
		return "~" + s
	}
	return s
}

func UsageTokensLine(contextPromptTok, lastUserPromptTok, reasoningTokens, responseTokens, totalTokens int64, outputTPS, ttftSecs, promptTPS float64, contextEstimated bool) string {
	var promptSeg string
	switch {
	case contextPromptTok <= 0 && lastUserPromptTok <= 0:
		promptSeg = WrapUser("0")
	case lastUserPromptTok <= 0:
		promptSeg = WrapContext(formatContextPromptTok(contextPromptTok, contextEstimated))
	case contextPromptTok <= 0:
		promptSeg = WrapUser(strconv.FormatInt(lastUserPromptTok, 10))
	default:
		promptSeg = WrapContext(formatContextPromptTok(contextPromptTok, contextEstimated)) + "+" + WrapUser(strconv.FormatInt(lastUserPromptTok, 10))
	}
	return "token: " + promptSeg + "+" +
		WrapThinking(strconv.FormatInt(reasoningTokens, 10)) + "+" +
		WrapAssistant(strconv.FormatInt(responseTokens, 10)) + "=" +
		WrapWhite(strconv.FormatInt(totalTokens, 10)) +
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
		_, err := io.WriteString(w.W, WrapTool(string(line)))
		return err
	}
	_, err := w.W.Write(line)
	return err
}
