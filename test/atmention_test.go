package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
)

func TestShortTagAndResolve(t *testing.T) {
	all := []atmention.Entry{
		{RelPath: "a/b/c.txt"},
		{RelPath: "d/b/c.txt"},
		{RelPath: "unique.go"},
	}
	if got := atmention.ShortTag("unique.go", all); got != "unique.go" {
		t.Fatalf("short unique: got %q", got)
	}
	if got := atmention.ShortTag("a/b/c.txt", all); got != "a/b/c.txt" {
		t.Fatalf("short disambiguated a: got %q", got)
	}
	if got := atmention.ShortTag("d/b/c.txt", all); got != "d/b/c.txt" {
		t.Fatalf("short disambiguated d: got %q", got)
	}
	e, ok := atmention.ResolveTag("a/b/c.txt", all)
	if !ok || e.RelPath != "a/b/c.txt" {
		t.Fatalf("resolve: %#v %v", e, ok)
	}
}

func TestAtTagBounds(t *testing.T) {
	line := []rune("see @a/b/c.txt and @unique.go end")
	bounds := atmention.TagRuneBounds(line)
	if len(bounds) != 2 {
		t.Fatalf("bounds: %d", len(bounds))
	}
	if got := atmention.JumpLeftOverTag(line, bounds[1].End); got != bounds[1].Start {
		t.Fatalf("jump left: %d", got)
	}
}

func TestMatchQueryREPrefixNotCursorNoise(t *testing.T) {
	entries := []atmention.Entry{
		{RelPath: ".cursor-video-frames", IsDir: true},
		{RelPath: "README.md"},
		{RelPath: "internal/agent/runtime/repl/editor.go"},
	}
	got := atmention.MatchQuery("RE", entries, 10)
	if len(got) == 0 {
		t.Fatal("expected matches for RE")
	}
	if got[0].RelPath == ".cursor-video-frames" {
		t.Fatalf("cursor dir must not rank first for RE, got %#v", got)
	}
	if got[0].RelPath != "README.md" {
		t.Fatalf("expected README first, got %#v", got)
	}
	if got := atmention.MatchQuery("R", []atmention.Entry{{RelPath: ".cursor-video-frames"}}, 1); len(got) != 0 {
		t.Fatalf("single R must not match .cursor-video-frames, got %#v", got)
	}
}

func TestMatchQueryPrefixOnSegment(t *testing.T) {
	entries := []atmention.Entry{{RelPath: "pkg/readme/foo.txt"}}
	got := atmention.MatchQuery("readme", entries, 5)
	if len(got) != 1 || got[0].RelPath != "pkg/readme/foo.txt" {
		t.Fatalf("segment prefix: %#v", got)
	}
	if got := atmention.MatchQuery("README", entries, 5); len(got) != 0 {
		t.Fatalf("case mismatch must not match, got %#v", got)
	}
}

func TestMatchQueryCaseSensitive(t *testing.T) {
	entries := []atmention.Entry{{RelPath: "README.md"}}
	if got := atmention.MatchQuery("re", entries, 5); len(got) != 0 {
		t.Fatalf("lowercase query must not match README.md, got %#v", got)
	}
	if got := atmention.MatchQuery("RE", entries, 5); len(got) != 1 || got[0].RelPath != "README.md" {
		t.Fatalf("exact-case prefix: %#v", got)
	}
}

func TestMatchQueryEmptyAndLimit(t *testing.T) {
	all := make([]atmention.Entry, 15)
	for i := range all {
		all[i] = atmention.Entry{RelPath: fmt.Sprintf("file%d.txt", i)}
	}
	if got := atmention.MatchQuery("", all, 10); len(got) != 0 {
		t.Fatalf("empty query should not match, got %d", len(got))
	}
	if got := atmention.MatchQuery("file", all, 10); len(got) != 10 {
		t.Fatalf("limit 10, got %d", len(got))
	}
}

func TestExpandLineFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	all := []atmention.Entry{{RelPath: "hello.txt"}}
	got, err := atmention.ExpandLine(context.Background(), "check @hello.txt", dir, all)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(got, "check @hello.txt", "--- file hello.txt ---", "hi") {
		t.Fatalf("expand: %q", got)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsStr(s, p) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
