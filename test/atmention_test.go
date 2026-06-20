package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestResolveTagAmbiguousReadmePicksRoot(t *testing.T) {
	all := []atmention.Entry{
		{RelPath: "README.md"},
		{RelPath: "docs/README.md"},
		{RelPath: "docs/architecture/README.md"},
	}
	e, ok := atmention.ResolveTag("README.md", all)
	if !ok || e.RelPath != "README.md" {
		t.Fatalf("root README: %#v %v", e, ok)
	}
	e, ok = atmention.ResolveTag("ReADME.md", all)
	if !ok || e.RelPath != "README.md" {
		t.Fatalf("case fold root README: %#v %v", e, ok)
	}
	e, ok = atmention.ResolveTag("docs/README.md", all)
	if !ok || e.RelPath != "docs/README.md" {
		t.Fatalf("nested README: %#v %v", e, ok)
	}
}

func TestExpandLineReadmeAtRoot(t *testing.T) {
	dir := t.TempDir()
	rootReadme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(rootReadme, []byte("root readme"), 0o600); err != nil {
		t.Fatal(err)
	}
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "README.md"), []byte("docs readme"), 0o600); err != nil {
		t.Fatal(err)
	}
	all := []atmention.Entry{
		{RelPath: "README.md"},
		{RelPath: "docs/README.md"},
	}
	got, err := atmention.ExpandLine(context.Background(), "count @README.md", dir, all)
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(got, "could not resolve") {
		t.Fatalf("should resolve: %q", got)
	}
	if !containsAll(got, "--- file README.md ---", "root readme") {
		t.Fatalf("expand root: %q", got)
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

func TestExpandLineFolderAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	all := []atmention.Entry{{RelPath: "pkg", IsDir: true}}
	got, err := atmention.ExpandLine(context.Background(), "see @pkg", dir, all)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(sub)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(got, "see @pkg", "--- folder pkg ---", abs) {
		t.Fatalf("folder expand: %q", got)
	}
}

func TestFindDocumentTagsSkipsCodeFence(t *testing.T) {
	src := "use @docs/a.md\n\n```\n@docs/b.md\n```\n\nalso `@docs/c.md`"
	tags := atmention.FindDocumentTags(src)
	if len(tags) != 1 || tags[0].Path != "docs/a.md" {
		t.Fatalf("tags: %#v", tags)
	}
}

func TestExpandDocumentIncludeRelative(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "child.md"), []byte("child body"), 0o600); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("see @docs/child.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := atmention.ExpandDocument(context.Background(), "see @docs/child.md", agents, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(got, "see @docs/child.md", "--- file docs/child.md ---", "child body") {
		t.Fatalf("document expand: %q", got)
	}
}

func TestExpandDocumentGitignoredSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".secret"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("x @.secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	n := atmention.NewNotifier()
	got, err := atmention.ExpandDocument(context.Background(), "x @.secret", agents, dir, n)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "--- file") {
		t.Fatalf("gitignored file should not expand: %q", got)
	}
	if got != "x @.secret" {
		t.Fatalf("unexpected expand output: %q", got)
	}
	msgs := n.Messages()
	if len(msgs) != 1 {
		t.Fatalf("notifier messages: %#v", msgs)
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
