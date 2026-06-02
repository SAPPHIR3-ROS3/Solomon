package test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
)

func TestReplComplete_shellPathBin(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"git", "go"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	env := agentruntime.ReplCompleteEnv{}
	line := []rune("!g")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("g") {
		t.Fatalf("offset=%d want 1", off)
	}
	seen := map[string]bool{}
	for _, s := range suffixes {
		seen[string(s)] = true
	}
	if !seen["it"] || !seen["o"] {
		t.Fatalf("suffixes=%v want git and go completions", suffixes)
	}
}

func TestReplComplete_goSubcommand(t *testing.T) {
	agentruntime.ReplCompleteResetGoCacheForTest()
	env := agentruntime.ReplCompleteEnv{}
	line := []rune("!go te")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("te") {
		t.Fatalf("offset=%d want 2 (go subcommand prefix)", off)
	}
	found := false
	for _, s := range suffixes {
		if string(s) == "st" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want test completion", suffixes)
	}
}

func TestReplComplete_shellPostPipe(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "grep")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	env := agentruntime.ReplCompleteEnv{}
	line := []rune("!echo hi | g")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("g") {
		t.Fatalf("offset=%d want 1 (command token prefix)", off)
	}
	found := false
	for _, s := range suffixes {
		if strings.HasPrefix("grep", "g"+string(s)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want grep completion", suffixes)
	}
}

func TestReplComplete_addSubcommand(t *testing.T) {
	env := agentruntime.ReplCompleteEnv{}
	line := []rune("/add ru")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("ru") {
		t.Fatalf("offset=%d want 2", off)
	}
	if len(suffixes) != 1 || string(suffixes[0]) != "le" {
		t.Fatalf("suffixes=%v want [le]", suffixes)
	}
}

func TestReplComplete_goSubcommandAfterGoSpace(t *testing.T) {
	agentruntime.ReplCompleteResetGoCacheForTest()
	env := agentruntime.ReplCompleteEnv{}
	line := []rune("!go ")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != 0 {
		t.Fatalf("offset=%d want 0 (empty go subcommand prefix)", off)
	}
	if len(suffixes) == 0 {
		t.Fatal("expected go subcommand candidates")
	}
}

func TestReplComplete_windowsPATHEXT(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PATHEXT matching is Windows-specific")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "tool.exe")
	if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".EXE")
	env := agentruntime.ReplCompleteEnv{}
	line := []rune("!too")
	suffixes, off := agentruntime.ReplCompleteDo(env, line, len(line))
	if off != len("too")-1 {
		t.Fatalf("offset=%d want partial command len", off)
	}
	found := false
	for _, s := range suffixes {
		if string(s) == "l" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("suffixes=%v want tool completion", suffixes)
	}
}
