package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestWelcomeBannerFitsBufferWidth(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cfg := &config.Root{
		Current: config.Current{Model: "qwen3.6-27b-mtp@q6_k"},
	}
	const termW = 72
	repl.PrintWelcomeBannerSized(&buf, termW, cfg, cfg.Current.Model, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "/Users/oni/Desktop/Projects/Golang/Solomon/with/a/very/long/path/segment", false, nil)
	const maxLineCells = termW
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		if plainLen(line) > maxLineCells {
			t.Fatalf("banner line wider than %d cells: %q", maxLineCells, line)
		}
	}
}

func TestChatSeparatorWidth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		termW int
		want  int
	}{
		{0, 24},
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 2},
		{72, 24},
		{120, 40},
	}
	for _, tc := range cases {
		if got := termcolor.ChatSeparatorWidth(tc.termW); got != tc.want {
			t.Fatalf("ChatSeparatorWidth(%d) = %d, want %d", tc.termW, got, tc.want)
		}
	}
}

func TestPrintChatSeparatorSized(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	termcolor.Init(termcolor.InitOptions{Out: &buf, ForceColor: true})
	const termW = 90
	termcolor.PrintChatSeparatorSized(&buf, termW)
	line := strings.TrimSpace(buf.String())
	want := termcolor.ChatSeparatorVisibleWidth(termW)
	if plainLen(line) != want {
		t.Fatalf("separator visible width = %d, want %d: %q", plainLen(line), want, line)
	}
	if !strings.Contains(line, "━") {
		t.Fatalf("expected heavy horizontal rule in %q", line)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[1;") && !strings.Contains(raw, "\x1b[1m") {
		t.Fatalf("expected bold ANSI in separator output: %q", raw)
	}
}

func plainLen(s string) int {
	n := 0
	esc := false
	for _, r := range s {
		if esc {
			if r == 'm' {
				esc = false
			}
			continue
		}
		if r == '\x1b' {
			esc = true
			continue
		}
		n++
	}
	return n
}
