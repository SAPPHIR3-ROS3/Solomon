package test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/replhl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/shelllex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestShelllexSegments(t *testing.T) {
	rs := []rune("git status | ls")
	segs := shelllex.Segments(rs)
	if len(segs) != 2 {
		t.Fatalf("segments=%d want 2", len(segs))
	}
	if len(segs[0].Words) < 1 || segs[0].Words[0].Text != "git" {
		t.Fatalf("first word=%q", segs[0].Words[0].Text)
	}
}

func initHLTestColors(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, ForceColor: true})
}

func TestReplHL_shellKnownUnknownCommand(t *testing.T) {
	dir := t.TempDir()
	writePATHExecutable(t, dir, "git")
	t.Setenv("PATH", dir)
	initHLTestColors(t)

	known := replhl.HighlightShell("git status", replhl.ShellEnv{})
	if termcolor.Plain(known) != "git status" {
		t.Fatalf("plain=%q", termcolor.Plain(known))
	}
	if !strings.Contains(known, "\x1b[") {
		t.Fatalf("expected ANSI for known command: %q", known)
	}

	unknown := replhl.HighlightShell("zzzznotcmd arg", replhl.ShellEnv{})
	if !strings.Contains(unknown, "\x1b[") {
		t.Fatalf("expected ANSI for unknown command: %q", unknown)
	}
}

func TestReplHL_shellGlobAndDollar(t *testing.T) {
	initHLTestColors(t)
	out := replhl.HighlightShell("ls *.go $HOME", replhl.ShellEnv{})
	if termcolor.Plain(out) != "ls *.go $HOME" {
		t.Fatalf("plain=%q", termcolor.Plain(out))
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI highlights: %q", out)
	}
}

func TestReplHL_shellComment(t *testing.T) {
	initHLTestColors(t)
	out := replhl.HighlightShell("echo ok # note", replhl.ShellEnv{})
	if termcolor.Plain(out) != "echo ok # note" {
		t.Fatalf("plain=%q", termcolor.Plain(out))
	}
	if strings.Count(out, "\x1b[") < 2 {
		t.Fatalf("expected multiple styles: %q", out)
	}
}

func TestReplHL_slashKnownUnknown(t *testing.T) {
	initHLTestColors(t)
	env := replcomplete.ReplCompleteEnv{}
	known := replhl.HighlightSlash("/help", env)
	if termcolor.Plain(known) != "/help" {
		t.Fatalf("plain=%q", termcolor.Plain(known))
	}
	if !strings.Contains(known, "\x1b[") {
		t.Fatalf("expected ANSI: %q", known)
	}
	if !strings.Contains(known, "\x1b[32m") && !strings.Contains(known, "\x1b[38;5;") {
		t.Fatalf("known slash command should use green (arg0): %q", known)
	}
	unknown := replhl.HighlightSlash("/zzzznotreal", env)
	if !strings.Contains(unknown, "\x1b[") {
		t.Fatalf("expected ANSI: %q", unknown)
	}
}

func TestReplHL_pathExists(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "foo.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	initHLTestColors(t)
	out := replhl.HighlightShell("cat ./foo.txt", replhl.ShellEnv{ProjRoot: root})
	if termcolor.Plain(out) != "cat ./foo.txt" {
		t.Fatalf("plain=%q", termcolor.Plain(out))
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected path highlight: %q", out)
	}
}

func TestReplHL_multilineShellFirst(t *testing.T) {
	initHLTestColors(t)
	env := replcomplete.ReplCompleteEnv{ReplShellFirst: true}
	lines := [][]rune{[]rune("ls"), []rune("!chat message")}
	shell := replhl.HighlightInputLine(lines, 0, true, env)
	if !strings.Contains(shell, "\x1b[") {
		t.Fatalf("shell line should be highlighted: %q", shell)
	}
	plain := replhl.HighlightInputLine(lines, 1, true, env)
	if plain != "!chat message" {
		t.Fatalf("chat line=%q want plain", plain)
	}
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("chat line should not be highlighted: %q", plain)
	}
}

func TestReplHL_multilineBangShell(t *testing.T) {
	initHLTestColors(t)
	env := replcomplete.ReplCompleteEnv{}
	lines := [][]rune{[]rune("!echo"), []rune("second")}
	line0 := replhl.HighlightInputLine(lines, 0, false, env)
	if !strings.HasPrefix(line0, "!") {
		t.Fatalf("bang prefix preserved: %q", line0)
	}
	if !strings.Contains(line0, "\x1b[") {
		t.Fatalf("shell highlight expected: %q", line0)
	}
	line1 := replhl.HighlightInputLine(lines, 1, false, env)
	if termcolor.Plain(line1) != "second" {
		t.Fatalf("plain=%q", termcolor.Plain(line1))
	}
}

func TestReplHL_noColorPlain(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: bytes.NewBuffer(nil), NoColor: true})
	out := replhl.HighlightShell("git *.go", replhl.ShellEnv{})
	if out != "git *.go" {
		t.Fatalf("got %q want plain", out)
	}
}

func TestReplHL_imgTagAfterHighlight(t *testing.T) {
	initHLTestColors(t)
	line := termcolor.ColorizeImgTagsReplInput("[img-1]")
	if !strings.Contains(line, "\x1b[") {
		t.Fatalf("img tag should be colorized: %q", line)
	}
	if termcolor.Plain(line) != "[img-1]" {
		t.Fatalf("plain=%q", termcolor.Plain(line))
	}
}

func TestReplcompletePathHighlightStatus(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	exists, prefix := replcomplete.PathHighlightStatus(root, "./a.go")
	if !exists || prefix {
		t.Fatalf("exists=%v prefix=%v", exists, prefix)
	}
	exists, prefix = replcomplete.PathHighlightStatus(root, "./a")
	if exists || !prefix {
		t.Fatalf("exists=%v prefix=%v want false true", exists, prefix)
	}
}

func TestShelllexCommandKnownPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH executable semantics differ on Windows in sandbox")
	}
	dir := t.TempDir()
	writePATHExecutable(t, dir, "mycmd")
	t.Setenv("PATH", dir)
	found, builtin := shelllex.CommandKnown("mycmd")
	if !found || builtin {
		t.Fatalf("found=%v builtin=%v", found, builtin)
	}
	found, _ = shelllex.CommandKnown("definitely-missing-cmd")
	if found {
		t.Fatal("expected missing command")
	}
}
