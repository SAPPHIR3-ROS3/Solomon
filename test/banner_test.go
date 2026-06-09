package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
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
