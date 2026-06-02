package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
)

func TestReplComplete_quotedPathWithSpace(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "my dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := agentruntime.ReplCompleteEnv{ProjRoot: root}
	line := []rune(`!cat "my dir/not`)
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("not") {
		t.Fatalf("offset=%d want len(not) for quoted path segment", off)
	}
	found := false
	for _, s := range suffixes {
		if string(s) == "es.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want notes.txt completion", suffixes)
	}
}

func TestReplComplete_backslashEscapedSpaceInPath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "my dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.log"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := agentruntime.ReplCompleteEnv{ProjRoot: root}
	line := []rune(`!cat my\ dir/da`)
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("da") {
		t.Fatalf("offset=%d want len(da)", off)
	}
	found := false
	for _, s := range suffixes {
		if string(s) == "ta.log" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want data.log completion", suffixes)
	}
}

func TestReplComplete_backslashPathSeparator(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "pkg", "lib.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := agentruntime.ReplCompleteEnv{ProjRoot: root}
	line := []rune(`!cat src\pkg\l`)
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("l") {
		t.Fatalf("offset=%d want 1", off)
	}
	if len(suffixes) != 1 || string(suffixes[0]) != "ib.go" {
		t.Fatalf("suffixes=%v want ib.go", suffixes)
	}
}

func TestReplComplete_dollarHOMEPath(t *testing.T) {
	home := t.TempDir()
	desktop := filepath.Join(home, "Desktop")
	if err := os.MkdirAll(filepath.Join(desktop, "Projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	env := agentruntime.ReplCompleteEnv{ProjRoot: t.TempDir()}
	line := []rune("!ls -lah $HOME/Desktop/P")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("P") {
		t.Fatalf("offset=%d want 1", off)
	}
	found := false
	for _, s := range suffixes {
		if strings.HasPrefix(string(s), "rojects") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want Projects completion", suffixes)
	}
}

func TestReplComplete_bracedHOMEPath(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	env := agentruntime.ReplCompleteEnv{}
	line := []rune("!ls ${HOME}/b")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("b") {
		t.Fatalf("offset=%d want 1", off)
	}
	if len(suffixes) == 0 {
		t.Fatal("expected completion for ${HOME}/bin")
	}
}

func TestReplComplete_tildeHomePath(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Desktop"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	env := agentruntime.ReplCompleteEnv{ProjRoot: t.TempDir()}
	line := []rune("!ls ~/Des")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("Des") {
		t.Fatalf("offset=%d want len(Des)", off)
	}
	found := false
	for _, s := range suffixes {
		suf := string(s)
		if strings.Contains(suf, "Desktop") {
			t.Fatalf("suffix %q should not repeat parent path segments", suf)
		}
		if strings.HasPrefix("ktop", suf) || strings.HasPrefix(suf, "ktop") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want Desktop completion (ktop)", suffixes)
	}
}

func TestReplComplete_absolutePathOutsideProjRoot(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(root, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	env := agentruntime.ReplCompleteEnv{ProjRoot: proj}
	partial := filepath.Join(root, "els")
	lineStr := "!ls " + partial
	line := []rune(lineStr)
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("els") {
		t.Fatalf("offset=%d want len(els)", off)
	}
	found := false
	for _, s := range suffixes {
		if strings.Contains(string(s), string(filepath.Separator)) && strings.Count(string(s), string(filepath.Separator)) > 1 {
			t.Fatalf("suffix %q looks like a multi-segment path", string(s))
		}
		if strings.HasPrefix(string(s), "ewhere") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want elsewhere completion", suffixes)
	}
}

func TestReplComplete_pathWithSpaceEscaped(t *testing.T) {
	home := t.TempDir()
	desktop := filepath.Join(home, "Desktop")
	photoEdit := filepath.Join(desktop, "Photo Edit")
	if err := os.MkdirAll(photoEdit, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	env := agentruntime.ReplCompleteEnv{ProjRoot: t.TempDir()}
	line := []rune("!ls -lah ~/Desktop/Photo")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("Photo") {
		t.Fatalf("offset=%d want len(Photo)", off)
	}
	found := false
	for _, s := range suffixes {
		got := string(s)
		if strings.Contains(got, "Desktop") {
			t.Fatalf("suffix %q must not repeat parent path", got)
		}
		if strings.Contains(got, `\ `) || strings.HasPrefix(got, `\ Edit`) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want escaped space before Edit", suffixes)
	}
}

func TestReplComplete_singleQuotedPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := agentruntime.ReplCompleteEnv{ProjRoot: root}
	line := []rune("!cat 'src/a")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("a") {
		t.Fatalf("offset=%d want 1", off)
	}
	found := false
	for _, s := range suffixes {
		if string(s) == "pp.go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want app.go completion", suffixes)
	}
}
