package test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/editor"
	"github.com/chzyer/readline"
)

func terminalInputPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = previous; r.Close(); w.Close() })
	return r, w
}

func TestREPLEOFEndsNormally(t *testing.T) {
	for _, input := range []string{"", "\x04"} {
		t.Run(fmt.Sprintf("input_%x", input), func(t *testing.T) {
			_, w := terminalInputPipe(t)
			if _, err := io.WriteString(w, input); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			rl, err := readline.NewEx(&readline.Config{Stdin: io.NopCloser(strings.NewReader("")), Stdout: io.Discard, Stderr: io.Discard})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := rl.Readline(); err != io.EOF {
				t.Fatalf("readline fixture did not reach EOF: %v", err)
			}
			defer rl.Close()
			if err := repl.Run(&repl.Loop{RL: rl, Out: io.Discard}); err != nil {
				t.Fatalf("EOF must end the REPL normally: %v", err)
			}
		})
	}
}

func TestEditorConsumesBufferedKeysWithoutMoreConsoleInput(t *testing.T) {
	_, w := terminalInputPipe(t)
	if _, err := io.WriteString(w, "hello\r"); err != nil {
		t.Fatal(err)
	}
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := editor.ReadMultiline(editor.Host{Out: io.Discard}, editor.NewHistory())
		done <- result{line, err}
	}()
	select {
	case got := <-done:
		if got.err != nil || got.line != "hello" {
			t.Fatalf("got %q, %v", got.line, got.err)
		}
	case <-time.After(2 * time.Second):
		w.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("buffered keys waited for additional console input")
	}
}

func TestEditorFinishRemovesGhostAndPreservesCheckpoint(t *testing.T) {
	for _, suffix := range []string{"esume", "esume\nphantom"} {
		t.Run(strings.ReplaceAll(suffix, "\n", "_"), func(t *testing.T) {
			h := repl.NewInputHistoryForTest()
			h.Add("!go t" + suffix)
			e := repl.NewMultilineEditorForTest(&repl.Loop{PromptPrimary: func() string { return "[#007] You: " }}, h, []string{"!go t"}, 0, 5, 80)
			e.RecomputeSuggest()
			if e.SuggestSuffix() == "" {
				t.Fatal("missing ghost fixture")
			}
			e.RefreshOutput()
			previousRows := e.RenderedRows()
			out := e.FinishOutput()
			if !strings.Contains(out, "[#007] You: ") || !strings.Contains(out, "\x1b[2K") {
				t.Fatalf("draft was not redrawn: %q", out)
			}
			if strings.Contains(out, "phantom") || strings.Contains(out, "esume") {
				t.Fatalf("ghost committed to terminal: %q", out)
			}
			if bytes.Count([]byte(out), []byte("\x1b[2K")) < previousRows {
				t.Fatalf("stale ghost rows left behind: %q", out)
			}
			if e.String() != "!go t" || e.RenderedRows() != 0 {
				t.Fatal("finish changed draft or left rendered state")
			}
		})
	}
}
