package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/editor"
)

func TestReplUndoRedoEdits(t *testing.T) {
	cases := []struct {
		name               string
		lines              []string
		row, col           int
		edit               func(*editor.MultilineEditorTest)
		want               string
		rowAfter, colAfter int
	}{
		{"unicode", []string{"ab"}, 0, 1, func(e *editor.MultilineEditorTest) { e.HandleRune('è') }, "aèb", 0, 2},
		{"newline", []string{"ab"}, 0, 1, func(e *editor.MultilineEditorTest) { e.HandleSequence("\x1b[13;2u") }, "a\nb", 1, 0},
		{"paste", []string{"ab"}, 0, 1, func(e *editor.MultilineEditorTest) { e.HandlePaste("è\r\nx\ry") }, "aè\nx\nyb", 2, 1},
		{"backspace", []string{"ab", "cd"}, 1, 0, func(e *editor.MultilineEditorTest) { e.HandleRune(127) }, "abcd", 0, 2},
		{"delete", []string{"ab", "cd"}, 0, 2, func(e *editor.MultilineEditorTest) { e.HandleSequence("\x1b[3~") }, "abcd", 0, 2},
		{"completion", []string{"/mo"}, 0, 3, func(e *editor.MultilineEditorTest) { e.HandleRune('\t') }, "/models", 0, 7},
		{"suggestion", []string{"/resu"}, 0, 5, func(e *editor.MultilineEditorTest) { e.RecomputeSuggest(); e.HandleRune(5) }, "/resume", 0, 7},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := editor.NewMultilineEditorForTest(editor.Host{}, nil, tt.lines, tt.row, tt.col, 80)
			tt.edit(e)
			if e.String() != tt.want || e.Row() != tt.rowAfter || e.Col() != tt.colAfter {
				t.Fatalf("edit: %q at %d:%d", e.String(), e.Row(), e.Col())
			}
			e.HandleRune(26)
			if e.String() != strings.Join(tt.lines, "\n") || e.Row() != tt.row || e.Col() != tt.col {
				t.Fatalf("undo: %q at %d:%d", e.String(), e.Row(), e.Col())
			}
			e.HandleRune(25)
			if e.String() != tt.want || e.Row() != tt.rowAfter || e.Col() != tt.colAfter {
				t.Fatalf("redo: %q at %d:%d", e.String(), e.Row(), e.Col())
			}
		})
	}
}

func TestReplUndoRedoBranchAndNavigation(t *testing.T) {
	e := editor.NewMultilineEditorForTest(editor.Host{}, nil, []string{""}, 0, 0, 80)
	e.HandleRune(26)
	e.HandleRune('a')
	e.HandleRune('b')
	e.HandleRune(26)
	e.HandleSequence("\x1b[D")
	e.HandleRune(25)
	if e.String() != "ab" || e.Col() != 2 {
		t.Fatalf("redo after movement: %q at %d", e.String(), e.Col())
	}
	e.HandleRune(26)
	e.HandleRune(127)
	e.HandleRune('c')
	e.HandleRune(25)
	if e.String() != "ca" {
		t.Fatalf("redo was not cleared: %q", e.String())
	}
	e.HandleSequence("\x1b[122;5u")
	e.HandleSequence("\x1b[122;6u")
	if e.String() != "ca" {
		t.Fatalf("extended keys: %q", e.String())
	}
}

func TestReplUndoHistoryIsolation(t *testing.T) {
	h := editor.NewHistoryForTest()
	h.Add("command")
	e := editor.NewMultilineEditorForTest(editor.Host{}, h, []string{"draft"}, 0, 5, 80)
	e.HandleRune('!')
	e.HandleRune(16)
	e.HandleRune(26)
	if e.String() != "command" {
		t.Fatalf("history undo crossed boundary: %q", e.String())
	}
	e.HandleRune(14)
	if e.String() != "draft!" {
		t.Fatalf("lost history draft: %q", e.String())
	}
	e.HandleRune('x')
	e.HandleRune(26)
	if e.String() != "draft!" {
		t.Fatalf("draft edit undo: %q", e.String())
	}
}

func TestReplUndoBounded(t *testing.T) {
	e := editor.NewMultilineEditorForTest(editor.Host{}, nil, []string{""}, 0, 0, 80)
	for i := 0; i < 120; i++ {
		e.HandleRune('x')
	}
	for i := 0; i < 120; i++ {
		e.HandleRune(26)
	}
	if len(e.String()) != 20 {
		t.Fatalf("undo bound: %d", len(e.String()))
	}
	for i := 0; i < 120; i++ {
		e.HandleRune(25)
	}
	if len(e.String()) != 120 {
		t.Fatalf("redo bound: %d", len(e.String()))
	}
}
