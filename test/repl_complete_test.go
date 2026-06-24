package test

import (
	"os"
	"path/filepath"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
)

func TestReplComplete_slashCommandPrefix(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("/mo")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if off != 2 {
		t.Fatalf("offset=%d want 2 (len of /mo)", off)
	}
	if len(suffixes) != 1 || string(suffixes[0]) != "dels" {
		t.Fatalf("suffixes=%v want [dels]", suffixes)
	}
}

func TestReplComplete_caseInsensitive(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("/MODELS")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if off != len("/MODELS")-1 {
		t.Fatalf("offset=%d want %d", off, len("/MODELS")-1)
	}
	if len(suffixes) != 1 || string(suffixes[0]) != "" {
		t.Fatalf("suffixes=%v want empty suffix for full match", suffixes)
	}
}

func TestReplComplete_unknownSlashToken(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	suffixes, off := replcomplete.ReplCompleteDo(env, []rune("/foo"), 4)
	if suffixes != nil || off != 0 {
		t.Fatalf("got suffixes=%v off=%d want nil,0", suffixes, off)
	}
}

func TestReplComplete_nonSlashLine(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	suffixes, off := replcomplete.ReplCompleteDo(env, []rune("hello"), 5)
	if suffixes != nil || off != 0 {
		t.Fatalf("got suffixes=%v off=%d want nil,0", suffixes, off)
	}
}

func TestReplComplete_commandOnlyNoArgComplete(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	suffixes, off := replcomplete.ReplCompleteDo(env, []rune("/agent arg"), len("/agent arg"))
	if suffixes != nil || off != 0 {
		t.Fatalf("got suffixes=%v off=%d want nil,0", suffixes, off)
	}
}

func TestReplComplete_reasoningArg(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("/reasoning l")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if off != len("l") {
		t.Fatalf("offset=%d want 1 (typed arg prefix)", off)
	}
	found := false
	for _, s := range suffixes {
		if string(s) == "ow" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want low completion", suffixes)
	}
}

func TestReplComplete_bangShellPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := replcomplete.ReplCompleteEnv{ProjRoot: root}
	line := []rune("!cat src/m")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if off != len("m") {
		t.Fatalf("offset=%d want 1 (last path segment prefix)", off)
	}
	found := false
	for _, s := range suffixes {
		if string(s) == "ain.go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want main.go completion", suffixes)
	}
}

func TestReplComplete_shellFirstPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "lib.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := replcomplete.ReplCompleteEnv{ProjRoot: root, ReplShellFirst: true}
	line := []rune("cat pkg/l")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if off != len("l") {
		t.Fatalf("offset=%d want 1 (last path segment prefix)", off)
	}
	if len(suffixes) != 1 || string(suffixes[0]) != "ib.go" {
		t.Fatalf("suffixes=%v", suffixes)
	}
}

func TestReplComplete_chatLineNoPathWithoutShellFirst(t *testing.T) {
	root := t.TempDir()
	env := replcomplete.ReplCompleteEnv{ProjRoot: root, ReplShellFirst: false}
	line := []rune("cat pkg/l")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if suffixes != nil || off != 0 {
		t.Fatalf("got suffixes=%v off=%d want nil,0", suffixes, off)
	}
}

func TestReplComplete_bangSlashNotSlashCommand(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("!/mo")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if suffixes != nil || off != 0 {
		t.Fatalf("got suffixes=%v off=%d want nil,0 for !/ (no slash completion)", suffixes, off)
	}
}

func TestReplComplete_logArg(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("/log wa")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if off != len("wa") {
		t.Fatalf("offset=%d want 2", off)
	}
	if len(suffixes) != 1 || string(suffixes[0]) != "rning" {
		t.Fatalf("suffixes=%v", suffixes)
	}
}

func TestReplComplete_midLineSlashCommand(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("please /hel")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if off != len("hel") {
		t.Fatalf("offset=%d want %d", off, len("hel"))
	}
	if len(suffixes) != 1 || string(suffixes[0]) != "p" {
		t.Fatalf("suffixes=%v want [p]", suffixes)
	}
}

func TestReplComplete_midLineSlashRejectedInPath(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("see foo/bar")
	suffixes, off := replcomplete.ReplCompleteDo(env, line, len(line))
	if suffixes != nil || off != 0 {
		t.Fatalf("got suffixes=%v off=%d want nil,0", suffixes, off)
	}
}

func TestSlashSuggestAt_midLine(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	line := []rune("plan with /resu")
	got := replcomplete.SlashSuggestAt(line, len(line), env, nil)
	if got != "plan with /resume" {
		t.Fatalf("slash suggest got %q", got)
	}
}

func TestReplInputPrefill_takeOnce(t *testing.T) {
	r := &agentruntime.Runtime{}
	r.SetReplInputPrefillForTest("skill body")
	if got := r.TakeReplInputPrefillForTest(); got != "skill body" {
		t.Fatalf("first take got %q", got)
	}
	if got := r.TakeReplInputPrefillForTest(); got != "" {
		t.Fatalf("second take got %q want empty", got)
	}
}
