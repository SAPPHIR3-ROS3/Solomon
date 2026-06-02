package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
)

func TestInputHistoryPrevNextKeepsDraft(t *testing.T) {
	h := repl.NewInputHistoryForTest()
	h.Add("one")
	h.Add("two\nline")

	got, ok := h.Prev("draft")
	if !ok || got != "two\nline" {
		t.Fatalf("prev got %q ok=%v", got, ok)
	}
	got, ok = h.Prev("ignored")
	if !ok || got != "one" {
		t.Fatalf("second prev got %q ok=%v", got, ok)
	}
	got, ok = h.Next()
	if !ok || got != "two\nline" {
		t.Fatalf("next got %q ok=%v", got, ok)
	}
	got, ok = h.Next()
	if !ok || got != "draft" {
		t.Fatalf("draft got %q ok=%v", got, ok)
	}
}

func TestMultilineEditorMovesVerticallyBeforeHistory(t *testing.T) {
	h := repl.NewInputHistoryForTest()
	h.Add("history")
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, h, []string{"line 1", "line 2", "line 3"}, 2, 4, 80)

	e.Up()
	if e.Row() != 1 || e.Col() != 4 || e.String() != "line 1\nline 2\nline 3" {
		t.Fatalf("up moved to row=%d col=%d text=%q", e.Row(), e.Col(), e.String())
	}
	e.Up()
	if e.Row() != 0 || e.Col() != 4 {
		t.Fatalf("second up row=%d col=%d", e.Row(), e.Col())
	}
	e.Up()
	if e.String() != "history" || e.Row() != 0 || e.Col() != len("history") {
		t.Fatalf("history load row=%d col=%d text=%q", e.Row(), e.Col(), e.String())
	}
}

func TestMultilineEditorCompletionUsesReplCompleter(t *testing.T) {
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, nil, []string{"/mo"}, 0, len("/mo"), 80)

	if !e.Complete() {
		t.Fatal("expected completion")
	}
	if got := e.Line(0); got != "/models" {
		t.Fatalf("completion got %q", got)
	}
}

func TestCommonRunePrefix(t *testing.T) {
	got := string(repl.CommonRunePrefixForTest([][]rune{[]rune("abc"), []rune("abd"), []rune("abz")}))
	if got != "ab" {
		t.Fatalf("prefix got %q", got)
	}
}
