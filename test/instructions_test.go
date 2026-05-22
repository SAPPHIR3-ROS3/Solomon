package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/instructions"
)

func TestInstructionsFindAgentsFilePriority(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, ok := instructions.FindAgentsFile(dir)
	if !ok || !strings.HasSuffix(p, "CLAUDE.md") {
		t.Fatalf("fallback: got %q ok=%v", p, ok)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, ok = instructions.FindAgentsFile(dir)
	if !ok || !strings.HasSuffix(p, "AGENTS.md") {
		t.Fatalf("priority: got %q ok=%v", p, ok)
	}
}

func TestInstructionsActivateFromPath(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "packages", "foo")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("pkg"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "bar.go")
	if err := os.WriteFile(file, []byte("package foo"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := instructions.ActivateDirsFromAbsPath(root, file)
	if len(got) != 1 || got[0] != "packages/foo" {
		t.Fatalf("activate: %v", got)
	}
	merged := instructions.MergeActivatedDirs(nil, got)
	if len(merged) != 1 || merged[0] != "packages/foo" {
		t.Fatalf("merge: %v", merged)
	}
}

func TestInstructionsShellPaths(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "src", "lib")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("lib"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := instructions.PathsFromShellCommand(root, "go test ./src/lib/...")
	if len(got) != 1 || got[0] != "src/lib" {
		t.Fatalf("shell paths: %v", got)
	}
}

func TestInstructionsRulesAddRemoveRenumber(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	projHex := "abc123"
	if _, err := instructions.AddRule(instructions.ScopeGlobal, "", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := instructions.AddRule(instructions.ScopeGlobal, "", "two"); err != nil {
		t.Fatal(err)
	}
	if _, err := instructions.AddRule(instructions.ScopeGlobal, "", "three"); err != nil {
		t.Fatal(err)
	}
	if err := instructions.RemoveRule(instructions.ScopeGlobal, "", 2); err != nil {
		t.Fatal(err)
	}
	rules, err := instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(rules))
	}
	if rules[0].Number != 1 || rules[0].Text != "one" {
		t.Fatalf("rule1: %+v", rules[0])
	}
	if rules[1].Number != 2 || rules[1].Text != "three" {
		t.Fatalf("rule2: %+v", rules[1])
	}
	if _, err := instructions.AddRule(instructions.ScopeProject, projHex, "proj rule"); err != nil {
		t.Fatal(err)
	}
	project, err := instructions.ListRules(instructions.ScopeProject, projHex)
	if err != nil || len(project) != 1 {
		t.Fatalf("project rules: %v err=%v", project, err)
	}
}

func TestInstructionsLoaderTruncation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(strings.Repeat("x", 100)), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := instructions.NewLoader()
	loader.MaxFileBytes = 40
	_, content, ok := loader.LoadRepoRoot(dir)
	if !ok {
		t.Fatal("want loaded file")
	}
	if !strings.Contains(content, "[truncated:") {
		t.Fatalf("footer missing: %q", content)
	}
}
