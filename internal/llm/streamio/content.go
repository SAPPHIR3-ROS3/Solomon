package streamio

import (
	"errors"
	"io"
	"regexp"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

var ErrLegacyEarlyStop = errors.New("legacy stream early stop")

var oversizedInlineSpaceRe = regexp.MustCompile(`([^\s]) {3,}([^\s])`)

func NormalizeReasoningWhitespace(s string) string {
	if s == "" {
		return ""
	}
	return oversizedInlineSpaceRe.ReplaceAllString(s, `$1 $2`)
}

func WriteReasoningDelta(sink io.Writer, s string) {
	if s == "" {
		return
	}
	_, _ = io.WriteString(sink, termcolor.WrapThinking(NormalizeReasoningWhitespace(s)))
}

func WriteContent(w io.Writer, s string) error {
	if s == "" {
		return nil
	}
	_, err := io.WriteString(w, s)
	return err
}

type legacyStreamTruncator interface {
	TruncatedContent() string
}

func WriteContentLegacy(w io.Writer, s string) (legacyStopped bool, err error) {
	err = WriteContent(w, s)
	if errors.Is(err, tooling.ErrLegacyToolBlockComplete) {
		if f, ok := w.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
		return true, nil
	}
	return false, err
}

func TruncatedContent(w io.Writer, fallback string) string {
	if t, ok := w.(legacyStreamTruncator); ok {
		if c := t.TruncatedContent(); c != "" {
			return c
		}
	}
	return fallback
}
