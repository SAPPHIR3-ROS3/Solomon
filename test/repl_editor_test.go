package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
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

func TestShellHistoryClassifiesWithoutBang(t *testing.T) {
	h := repl.NewInputHistoryForTest()
	h.AddWithMode("!go test ./...", false)
	if got := h.ShellMatch("go t"); got != "go test ./..." {
		t.Fatalf("shell match got %q", got)
	}
}

func TestShellPrefixNormalizedStripsBang(t *testing.T) {
	got := repl.ShellPrefixNormalizedForTest("!go test", false)
	if got != "go test" {
		t.Fatalf("normalized got %q", got)
	}
	if got := repl.ShellPrefixNormalizedForTest("hello", false); got != "" {
		t.Fatalf("chat normalized got %q", got)
	}
}

func TestAutosuggestShellFromHistory(t *testing.T) {
	h := repl.NewInputHistoryForTest()
	h.AddWithMode("!go test ./...", false)
	e := repl.NewMultilineEditorForTest(&repl.Loop{CompleteEnv: replcomplete.ReplCompleteEnv{}}, h, []string{"!go t"}, 0, len("!go t"), 80)
	e.RecomputeSuggest()
	if got := e.SuggestSuffix(); got != "est ./..." {
		t.Fatalf("ghost suffix got %q", got)
	}
	e.AcceptSuggestAll()
	if got := e.String(); got != "!go test ./..." {
		t.Fatalf("accepted got %q", got)
	}
}

func TestSlashSuggestUnique(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	got := replcomplete.SlashSuggest(env, "/res", nil)
	if got != "/resume" {
		t.Fatalf("slash suggest got %q", got)
	}
}

func TestSlashSuggestAmbiguousUsesHistory(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	hist := []string{"/new", "/reasoning med"}
	got := replcomplete.SlashSuggest(env, "/re", hist)
	if got != "/reasoning med" {
		t.Fatalf("slash suggest got %q", got)
	}
}

func TestSlashSuggestAmbiguousNoHistory(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	got := replcomplete.SlashSuggest(env, "/re", nil)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestGhostVisualRowsSameLine(t *testing.T) {
	prompt := "[#000] You: "
	line := "/r"
	ghost := "esume"
	separate := repl.VisualRowsWithGhostForTest(80, prompt, line, "") +
		repl.VisualRowsWithGhostForTest(80, "", ghost, "")
	combined := repl.VisualRowsWithGhostForTest(80, prompt, line, ghost)
	if combined > separate {
		t.Fatalf("combined rows %d should not exceed separate sum %d", combined, separate)
	}
	if combined != 1 {
		t.Fatalf("expected 1 visual row, got %d", combined)
	}
}

func TestAutosuggestSlashGhost(t *testing.T) {
	e := repl.NewMultilineEditorForTest(&repl.Loop{CompleteEnv: replcomplete.ReplCompleteEnv{}}, nil, []string{"/res"}, 0, len("/res"), 80)
	e.RecomputeSuggest()
	if got := e.SuggestSuffix(); got != "ume" {
		t.Fatalf("ghost got %q", got)
	}
}
