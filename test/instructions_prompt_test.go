package test

import (
	"context"
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
	sections, err := loader.BuildPromptSections(context.Background(), root, "hex", nil, nil)
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
	empty, err := prompt.RenderAgent(prompt.Data{Tools: "t", Syntax: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(empty, "## Custom rules") || strings.Contains(empty, "## Global instructions") {
		t.Fatalf("empty sections should be omitted: %q", empty)
	}
	withSections, err := prompt.RenderAgent(prompt.Data{
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

func TestImagesWorkflowSection_inPrompts(t *testing.T) {
	section := prompt.ImagesWorkflowSection()
	if section == "" {
		t.Fatal("empty images workflow section")
	}
	if strings.Contains(section, "[img-") {
		t.Fatalf("section must avoid bracket img literals stripped before API: %q", section)
	}
	for _, sub := range []string{"SHA-256", "ImageFiles", "U+200B", "private-use", "Ctrl+V"} {
		if !strings.Contains(section, sub) {
			t.Fatalf("missing %q in section", sub)
		}
	}
	agent, err := prompt.RenderAgent(prompt.Data{Tools: "t", Syntax: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(agent, "## Session images") {
		t.Fatalf("agent prompt missing images section")
	}
	chat, err := prompt.RenderChat(prompt.Data{Tools: "t", Syntax: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chat, "## Session images") {
		t.Fatalf("chat prompt missing images section")
	}
	if !strings.Contains(agent, "PLAN mode cannot paste") {
		t.Fatalf("images workflow missing plan-specific note: %q", agent)
	}
	sumSys, err := prompt.RenderSummarizeSystem(prompt.SummarizeData{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sumSys, "wire token") {
		t.Fatalf("summarize system missing wire token description")
	}
}

func TestRenderAgent_AnonymizeOmitsSolomon(t *testing.T) {
	got, err := prompt.RenderAgent(prompt.Data{
		Tools:     "t",
		Syntax:    prompt.AnonymizeNativeToolInvocationSyntax(),
		Anonymize: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"solomon", "harness", "you operate in agent mode"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("compat prompt must not mention %q:\n%s", forbidden, got)
		}
	}
	if !strings.Contains(got, "coding assistant") {
		t.Fatalf("compat prompt missing neutral intro: %q", got)
	}
}
