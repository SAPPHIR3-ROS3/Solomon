package test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func TestSlashActivateDeactivateRulesAndSkills(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	var output bytes.Buffer
	deps := testDeps(nil)
	deps.Out = &output
	if err := agent.SlashDispatch(deps, "/add rule global habit"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(deps, "/deactivate rule 1"); err != nil {
		t.Fatal(err)
	}
	text, err := instructions.LoadAllRulesText(deps.ProjHex)
	if err != nil || strings.Contains(text, "global habit") {
		t.Fatalf("inactive global rule in prompt: %q err=%v", text, err)
	}
	if err := agent.SlashDispatch(deps, "/rules"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "[inactive] global habit") {
		t.Fatalf("inactive rule not listed: %q", output.String())
	}
	if err := agent.SlashDispatch(deps, "/activate rule 1"); err != nil {
		t.Fatal(err)
	}
	text, err = instructions.LoadAllRulesText(deps.ProjHex)
	if err != nil || !strings.Contains(text, "global habit") {
		t.Fatalf("active global rule missing from prompt: %q err=%v", text, err)
	}
	if err := agent.SlashDispatch(deps, "/add projectrule project habit"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(deps, "/deactivate projectrule 1"); err != nil {
		t.Fatal(err)
	}
	text, err = instructions.LoadAllRulesText(deps.ProjHex)
	if err != nil || strings.Contains(text, "project habit") {
		t.Fatalf("inactive project rule in prompt: %q err=%v", text, err)
	}
	if err := agent.SlashDispatch(deps, "/activate projectrule 1"); err != nil {
		t.Fatal(err)
	}

	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	globalClone := filepath.Join(t.TempDir(), "global-skill")
	localClone := filepath.Join(paths.LocalSkillsDir(deps.ProjRoot), "local-skill")
	reg.Global["global-key"] = skills.SkillEntry{Name: "Shared Skill", ClonePath: globalClone}
	reg.Projects[deps.ProjHex] = map[string]skills.SkillEntry{"local-key": {Name: "Shared Skill", ClonePath: localClone}}
	if err := skills.SaveRegistry(regPath, reg); err != nil {
		t.Fatal(err)
	}
	deps.Session().Capabilities.Capture("loadSkill", capabilityJSON(t, map[string]string{"name": "Shared Skill", "slash": "shared-skill", "body": "loaded body"}))
	if len(deps.Session().Capabilities.LoadedSkills) != 1 {
		t.Fatal("skill fixture was not captured")
	}
	if err := agent.SlashDispatch(deps, "/deactivate skill Shared Skill"); err == nil || !strings.Contains(err.Error(), "multiple skills") {
		t.Fatalf("ambiguous skill should require scope: %v", err)
	}
	if err := agent.SlashDispatch(deps, "/deactivate skill Shared Skill --scope global"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(deps, "/deactivate skill Shared Skill --scope local"); err != nil {
		t.Fatal(err)
	}
	if len(deps.Session().Capabilities.LoadedSkills) != 0 || !deps.Session().Capabilities.DisabledSkills["shared skill"] {
		t.Fatalf("deactivated skill remained in session capabilities: %+v", deps.Session().Capabilities)
	}
	reg, err = skills.LoadRegistry(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reg.Global["global-key"].Disabled || !reg.Projects[deps.ProjHex]["local-key"].Disabled {
		t.Fatalf("disabled skill states not saved: %+v", reg)
	}
	refs := skills.OrderedSkillRefs(reg, deps.ProjHex, deps.ProjRoot)
	if len(refs) != 0 {
		t.Fatalf("disabled skills still discoverable: %+v", refs)
	}
	if err := agent.SlashDispatch(deps, "/activate skill Shared Skill --scope local"); err != nil {
		t.Fatal(err)
	}
	reg, err = skills.LoadRegistry(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Projects[deps.ProjHex]["local-key"].Disabled {
		t.Fatal("local skill did not reactivate")
	}
}
