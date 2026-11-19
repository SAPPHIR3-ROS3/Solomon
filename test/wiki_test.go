package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func wikiWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func wikiRead(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func wikiRun(docs, output string, extra ...string) (string, error) {
	args := []string{"run", "../scripts/wiki", "--docs", docs, "--output", output, "--repository", "owner/repo"}
	result, err := exec.Command("go", append(args, extra...)...).CombinedOutput()
	return string(result), err
}

func TestWikiMirror(t *testing.T) {
	root := t.TempDir()
	docs, output := filepath.Join(root, "docs"), filepath.Join(root, "wiki")
	wikiWrite(t, filepath.Join(docs, "README.md"), "# Home\n[Guide](guide/README.md#setup)\n[Ref]: guide/A%20B.MD?q=1#frag \"Title\"\n<a href=\"../assets\"><img src=\"image.svg\"></a>")
	fence := strings.Repeat("\x60", 3)
	wikiWrite(t, filepath.Join(docs, "guide", "README.md"), "# Setup\n[Home](../README.md)\n[Code](../../code.go)\n![Image](../image.svg)\n[Remote](https://example.com)\n[Anchor](#setup)\n"+fence+"\n[Example](../README.md)\n"+fence)
	wikiWrite(t, filepath.Join(root, "assets", "keep"), "keep")
	wikiWrite(t, filepath.Join(docs, "image.svg"), "<svg/>")
	wikiWrite(t, filepath.Join(output, ".git", "keep"), "keep")
	wikiWrite(t, filepath.Join(output, "obsolete.md"), "old")
	render := func() {
		t.Helper()
		if result, err := wikiRun(docs, output, "--revision", "abc"); err != nil || result != "Files mirrored: 3\n" {
			t.Fatalf("mirror: %v: %s", err, result)
		}
	}
	render()
	if _, err := os.Stat(filepath.Join(output, "obsolete.md")); !os.IsNotExist(err) {
		t.Fatalf("obsolete page was not removed: %v", err)
	}
	if got := wikiRead(t, filepath.Join(output, ".git", "keep")); got != "keep" {
		t.Errorf(".git marker = %q", got)
	}
	if got := wikiRead(t, filepath.Join(output, "image.svg")); got != "<svg/>" {
		t.Errorf("asset = %q", got)
	}
	home := wikiRead(t, filepath.Join(output, "Home.md"))
	for _, want := range []string{"(guide--README#setup)", "[Ref]: guide--A%20B?q=1#frag \"Title\"", "href=\"https://github.com/owner/repo/tree/abc/assets\"", "src=\"image.svg\""} {
		if !strings.Contains(home, want) {
			t.Errorf("home page missing %q: %q", want, home)
		}
	}
	guide := filepath.Join(output, "guide--README.md")
	text := wikiRead(t, guide)
	for _, want := range []string{"[Home](Home)", "/blob/abc/code.go", "(image.svg)", "(https://example.com)", "(#setup)", "[Example](../README.md)"} {
		if !strings.Contains(text, want) {
			t.Errorf("guide page missing %q: %q", want, text)
		}
	}
	render()
	if got := wikiRead(t, guide); got != text {
		t.Errorf("rerun changed guide: %q", got)
	}
}

func TestWikiCollisionAndOverlap(t *testing.T) {
	root := t.TempDir()
	docs, output := filepath.Join(root, "docs"), filepath.Join(root, "wiki")
	wikiWrite(t, filepath.Join(docs, "README.md"), "# Index")
	wikiWrite(t, filepath.Join(docs, "Home.md"), "# Collision")
	marker := filepath.Join(output, "keep.md")
	wikiWrite(t, marker, "keep")
	if result, err := wikiRun(docs, output); err == nil || !strings.Contains(result, "wiki page collision") {
		t.Fatalf("expected collision, got %v: %s", err, result)
	}
	if got := wikiRead(t, marker); got != "keep" {
		t.Errorf("marker after collision = %q", got)
	}
	if result, err := wikiRun(docs, filepath.Join(docs, "out")); err == nil || !strings.Contains(result, "source and output must not overlap") {
		t.Fatalf("expected overlap, got %v: %s", err, result)
	}
}
