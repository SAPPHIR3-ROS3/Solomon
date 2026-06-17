package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestMultilineEditorCompletionCyclesOnTab(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "foo1.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "foo2.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := replcomplete.ReplCompleteEnv{ProjRoot: root}
	e := repl.NewMultilineEditorForTest(&repl.Loop{CompleteEnv: env}, nil, []string{"!cat foo"}, 0, len("!cat foo"), 80)
	if !e.Complete() {
		t.Fatal("expected first completion")
	}
	if got := e.Line(0); got != "!cat foo1.txt" {
		t.Fatalf("first completion got %q", got)
	}
	if !e.Complete() {
		t.Fatal("expected second completion")
	}
	if got := e.Line(0); got != "!cat foo2.txt" {
		t.Fatalf("second completion got %q", got)
	}
	if !e.Complete() {
		t.Fatal("expected third completion")
	}
	if got := e.Line(0); got != "!cat foo1.txt" {
		t.Fatalf("cycled completion got %q", got)
	}
}

func TestMultilineEditorRendersAllLogicalLines(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i)
	}
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, nil, lines, 39, len(lines[39]), 80)
	if got := e.TotalVisualRows(); got != 40 {
		t.Fatalf("visual rows got %d want 40", got)
	}
	for i := 0; i < 30; i++ {
		e.Up()
	}
	if got := e.Row(); got != 9 {
		t.Fatalf("row got %d want 9", got)
	}
}

func TestMultilineEditorPasteViewportKeepsFullBufferAndNavigates(t *testing.T) {
	h := repl.NewInputHistoryForTest()
	h.Add("history")
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i)
	}
	paste := strings.Join(lines, "\n")
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, h, []string{""}, 0, 0, 80)
	e.SetHeight(6)
	e.InsertPaste(paste)
	e.RefreshOutput()
	if e.String() != paste {
		t.Fatalf("paste changed buffer: %q", e.String())
	}
	if e.RenderedRows() != 5 {
		t.Fatalf("rendered rows got %d want 5", e.RenderedRows())
	}
	if e.ViewportTop() == 0 {
		t.Fatal("expected viewport to follow cursor near end")
	}
	for i := 0; i < len(lines)-1; i++ {
		e.Up()
	}
	if e.Row() != 0 || e.String() != paste {
		t.Fatalf("up through paste row=%d text=%q", e.Row(), e.String())
	}
	e.RefreshOutput()
	if e.ViewportTop() != 0 {
		t.Fatalf("viewport top got %d want 0", e.ViewportTop())
	}
	e.Up()
	if e.String() != "history" {
		t.Fatalf("history loaded before first logical row: %q", e.String())
	}
}

func TestMultilineEditorTypingAutoWrapsAtSpace(t *testing.T) {
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, nil, []string{""}, 0, 0, 12)
	e.TypeString("hi foob")
	if got := e.String(); got != "hi \nfoob" {
		t.Fatalf("auto wrap got %q want %q", got, "hi \nfoob")
	}
}

func TestMultilineEditorTypingLongWordStaysOnOneLine(t *testing.T) {
	text := strings.Repeat("x", 30)
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, nil, []string{""}, 0, 0, 15)
	e.TypeString(text)
	got := e.String()
	if got != text {
		t.Fatalf("typed text got %q want %q", got, text)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("typing inserted logical newline without split point: %q", got)
	}
	if got := e.TotalVisualRows(); got <= 1 {
		t.Fatalf("visual rows got %d want wrapped rows", got)
	}
}

func TestMultilineEditorRefreshShrinksRenderedRows(t *testing.T) {
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, nil, []string{"one", "two", "three"}, 2, 0, 80)
	e.RefreshOutput()
	if e.RenderedRows() != 3 {
		t.Fatalf("initial rendered rows got %d want 3", e.RenderedRows())
	}
	out := e.RefreshOutput()
	if strings.Count(out, "\x1b[2K") != 3 {
		t.Fatalf("initial clear count got %d want 3", strings.Count(out, "\x1b[2K"))
	}
	e.Backspace()
	out = e.RefreshOutput()
	if e.RenderedRows() != 2 {
		t.Fatalf("rendered rows after shrink got %d want 2", e.RenderedRows())
	}
	if strings.Count(out, "\x1b[2K") < 3 {
		t.Fatalf("expected shrink refresh to clear stale tail, output %q", out)
	}
}

func TestMultilineEditorFinishPrintsFullViewportedDraft(t *testing.T) {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i)
	}
	e := repl.NewMultilineEditorForTest(&repl.Loop{}, nil, []string{""}, 0, 0, 80)
	e.SetHeight(5)
	e.InsertPaste(strings.Join(lines, "\n"))
	e.RefreshOutput()
	if e.ViewportTop() == 0 {
		t.Fatal("expected viewport before finish")
	}
	out := e.FinishOutput()
	if !strings.Contains(out, "line 00") || !strings.Contains(out, "line 11") {
		t.Fatalf("finish did not print full draft: %q", out)
	}
	if e.RenderedRows() != 0 || e.ViewportTop() != 0 || e.CursorLine() != 0 {
		t.Fatalf("finish left state rendered=%d viewport=%d cursor=%d", e.RenderedRows(), e.ViewportTop(), e.CursorLine())
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
	got := replcomplete.SlashSuggest(env, "/resu", nil)
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
	e := repl.NewMultilineEditorForTest(&repl.Loop{CompleteEnv: replcomplete.ReplCompleteEnv{}}, nil, []string{"/resu"}, 0, len("/resu"), 80)
	e.RecomputeSuggest()
	if got := e.SuggestSuffix(); got != "me" {
		t.Fatalf("ghost got %q", got)
	}
}
