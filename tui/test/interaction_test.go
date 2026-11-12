package test

import (
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/interaction"
)

func interactionWidth(s string) int {
	width := 0
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r == '界' || r == '🙂':
			width += 2
		default:
			width++
		}
	}
	return width
}

func TestInteractionGeometry(t *testing.T) {
	cases := []struct {
		name, view, clean string
		hits              map[[2]int]string
	}{
		{"single", "   " + interaction.Mark("a", "abc") + "!", "   abc!", map[[2]int]string{{2, 0}: "", {3, 0}: "a", {5, 0}: "a", {6, 0}: ""}},
		{"multiline", "  " + interaction.Mark("a", "ab\ncde\n\nf"), "  ab\ncde\n\nf", map[[2]int]string{{2, 0}: "a", {0, 1}: "a", {3, 1}: "", {0, 2}: "", {0, 3}: "a"}},
		{"ansi", "\x1b[31m  " + interaction.Mark("a", "x\x1b[0my"), "\x1b[31m  x\x1b[0my", map[[2]int]string{{1, 0}: "", {2, 0}: "a", {3, 0}: "a", {4, 0}: ""}},
		{"unicode", "界e\u0301" + interaction.Mark("a", "🙂界"), "界e\u0301🙂界", map[[2]int]string{{2, 0}: "", {3, 0}: "a", {6, 0}: "a", {7, 0}: ""}},
		{"nested", interaction.Mark("outer", "a"+interaction.Mark("inner", "bc")+"d"), "abcd", map[[2]int]string{{0, 0}: "outer", {1, 0}: "inner", {2, 0}: "inner", {3, 0}: "outer"}},
		{"empty", interaction.Mark("empty", ""), "", map[[2]int]string{{0, 0}: ""}},
		{"emptyAnsi", interaction.Mark("empty", "\x1b[0m"), "\x1b[0m", map[[2]int]string{{0, 0}: ""}},
		{"carriage", interaction.Mark("a", "ab\r\ncd"), "ab\r\ncd", map[[2]int]string{{1, 0}: "a", {1, 1}: "a", {2, 1}: ""}},
		{"bareCarriage", "padding\r" + interaction.Mark("a", "x"), "padding\rx", map[[2]int]string{{0, 0}: "a", {1, 0}: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m interaction.Map
			if got := m.Capture(tc.view, interactionWidth); got != tc.clean {
				t.Fatalf("clean = %q, want %q", got, tc.clean)
			}
			for point, want := range tc.hits {
				if got := m.Hit(point[0], point[1]); got != want {
					t.Errorf("Hit%v = %q, want %q", point, got, want)
				}
			}
			if m.Hit(-1, 0) != "" || m.Hit(0, -1) != "" {
				t.Fatal("negative coordinates matched")
			}
			m.Capture("plain", interactionWidth)
			if m.Hit(0, 0) != "" || m.Hit(3, 0) != "" {
				t.Fatal("capture did not reset regions")
			}
		})
	}
}

func TestInteractionMarkers(t *testing.T) {
	var m interaction.Map
	scroll := "\x1b]777;scroll;12;4\x1b\\"
	link := "\x1b]8;;https://example.org\a"
	start := "\x1b]777;zone;start;a\x1b\\"
	end := "\x1b]777;zone;end;a\x1b\\"
	for _, tc := range []struct{ view, clean, hit string }{
		{end + "text", "text", ""},
		{start + "text", "text", "a"},
		{start, "", ""},
		{start + "x" + end + end, "x", "a"},
		{scroll + interaction.Mark("a", link+"x"), scroll + link + "x", "a"},
		{"\x1b]777;zone;start;a\ax\x1b]777;zone;end;a\a", "x", "a"},
	} {
		if got := m.Capture(tc.view, interactionWidth); got != tc.clean {
			t.Fatalf("clean = %q, want %q", got, tc.clean)
		}
		if got := m.Hit(0, 0); got != tc.hit {
			t.Fatalf("hit = %q, want %q", got, tc.hit)
		}
	}
	for _, id := range []string{"", "a b", "é", "a;b", "a\n", "a\x1b"} {
		if got := interaction.Mark(id, "body"); got != "body" {
			t.Fatalf("invalid ID %q accepted", id)
		}
	}
	for _, id := range []string{"a", "AZaz09_-:.", "panel.row:1"} {
		marked := interaction.Mark(id, "x")
		if marked == "x" {
			t.Fatalf("valid ID %q rejected", id)
		}
		if m.Capture(marked, interactionWidth) != "x" || m.Hit(0, 0) != id {
			t.Fatalf("valid ID %q failed", id)
		}
	}
	invalid := "\x1b]777;zone;start;bad id\x1b\\"
	if m.Capture(invalid+"x", interactionWidth) != invalid+"x" {
		t.Fatal("non-marker OSC changed")
	}
}

func TestInteractionRecencyAndCost(t *testing.T) {
	var m interaction.Map
	view := "\x1b]777;zone;start;a\x1b\\x\x1b]777;zone;start;b\x1b\\y\x1b]777;zone;end;a\x1b\\z"
	m.Capture(view, interactionWidth)
	if m.Hit(0, 0) != "a" || m.Hit(1, 0) != "b" || m.Hit(2, 0) != "b" {
		t.Fatal("opening order not respected")
	}
	calls := 0
	measure := func(s string) int { calls++; return utf8.RuneCountInString(s) }
	m.Capture(interaction.Mark("a", strings.Repeat("x", 100000)), measure)
	if calls > 3 {
		t.Fatalf("width measured %d times for one line", calls)
	}
	m.Capture(interaction.Mark("a", "x"), nil)
	if m.Hit(0, 0) != "a" {
		t.Fatal("nil measure fallback failed")
	}
}

func TestInteractionConcurrent(t *testing.T) {
	var m interaction.Map
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Capture(interaction.Mark("a", "x\ny"), interactionWidth)
				m.Hit(0, 0)
			}
		}()
	}
	wg.Wait()
	if m.Hit(0, 1) != "a" {
		t.Fatal("concurrent capture lost regions")
	}
}
