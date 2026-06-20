package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestPrintBtwSeparatorSized(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	termcolor.Init(termcolor.InitOptions{Out: &buf, ForceColor: true})
	const termW = 90
	termcolor.PrintBtwSeparatorSized(&buf, termW)
	line := strings.TrimSpace(buf.String())
	want := termcolor.ChatSeparatorVisibleWidth(termW)
	if plainLen(line) != want {
		t.Fatalf("separator visible width = %d, want %d: %q", plainLen(line), want, line)
	}
	if !strings.Contains(line, "━") {
		t.Fatalf("expected heavy horizontal rule in %q", line)
	}
	raw := buf.String()
	if strings.Contains(raw, "\x1b[1;") || strings.Contains(raw, "\x1b[1m") {
		t.Fatalf("btw separator should not use bold gold styling: %q", raw)
	}
}
