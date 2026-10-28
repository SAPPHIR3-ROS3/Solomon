package test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/charmbracelet/x/ansi"
)

func TestWelcomeBannerCustomLogoFitsTerminal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	for _, dir := range []string{"ascii", "colors"} {
		if err := os.MkdirAll(filepath.Join(home, "banner", dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var colorOut bytes.Buffer
	termcolor.Init(termcolor.InitOptions{Out: &colorOut, ForceColor: true})
	t.Cleanup(func() { termcolor.Init(termcolor.InitOptions{Out: &colorOut, NoColor: true}) })
	for _, art := range []string{"CUSTOM\nLOGO\nEND", strings.Repeat("X", 160), strings.Repeat("王", 80), strings.Repeat("e\u0301", 160), strings.Repeat("👩‍💻", 80), strings.Repeat("X\n", 64)} {
		if err := os.WriteFile(filepath.Join(home, "banner", "ascii", "logo.txt"), []byte(art), 0600); err != nil {
			t.Fatal(err)
		}
		var colorRows []string
		for _, row := range strings.Split(strings.TrimRight(art, "\n"), "\n") {
			colorRows = append(colorRows, strings.TrimSpace(strings.Repeat("FF0000 ", len([]rune(row)))))
		}
		if err := os.WriteFile(filepath.Join(home, "banner", "colors", "colors.txt"), []byte(strings.Join(colorRows, "\n")), 0600); err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{1, 4, 5, 10, 20, 40, 72, 120, 240} {
			var buf bytes.Buffer
			repl.PrintWelcomeBannerSized(&buf, width, nil, "model", "test-project", home, false, nil)
			for _, line := range strings.Split(buf.String(), "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Fatalf("banner line %d cells exceeds terminal width %d: %q", got, width, line)
				}
			}
			if width == 240 && strings.HasPrefix(art, "CUSTOM") && !strings.Contains(ansi.Strip(buf.String()), "END") {
				t.Fatal("banner should include every custom logo row")
			}
		}
	}
}

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
	// termcolor.Init changes the process-wide renderer; keep this test serial.
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
