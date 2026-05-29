package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
)

func TestInstructionsPromptSectionsConditional(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "AGENTS.md"), []byte("global agents"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root agents"), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := instructions.NewLoader()
	sections, err := loader.BuildPromptSections(root, "hex", nil)
	if err != nil {
		t.Fatal(err)
	}
	if sections.CustomRules != "" {
		t.Fatalf("unexpected custom rules: %q", sections.CustomRules)
	}
	if !strings.Contains(sections.GlobalInstructions, "global agents") {
		t.Fatalf("global: %q", sections.GlobalInstructions)
	}
	if !strings.Contains(sections.RepoInstructions, "root agents") {
		t.Fatalf("repo: %q", sections.RepoInstructions)
	}
	empty, err := prompt.RenderBuild(prompt.Data{Tools: "t", Syntax: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(empty, "## Custom rules") || strings.Contains(empty, "## Global instructions") {
		t.Fatalf("empty sections should be omitted: %q", empty)
	}
	withSections, err := prompt.RenderBuild(prompt.Data{
		Tools:              "t",
		Syntax:             "s",
		GlobalInstructions: sections.GlobalInstructions,
		RepoInstructions:   sections.RepoInstructions,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withSections, "## Global instructions") || !strings.Contains(withSections, "## Repository instructions") {
		t.Fatalf("sections missing: %q", withSections)
	}
}
