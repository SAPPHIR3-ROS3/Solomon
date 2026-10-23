package test

import (
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func TestSkillActivationPersistsAndFiltersDiscovery(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	projRoot := t.TempDir()
	projHex := "project-id"
	globalKey := "global-key"
	projectKey := "project-key"
	localKey := "local-key"
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	projectSkillsDir, err := paths.ProjectSkillsDir(projHex)
	if err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	reg.Global[globalKey] = skills.SkillEntry{Name: "Shared", ClonePath: filepath.Join("global", globalKey)}
	reg.Projects[projHex] = map[string]skills.SkillEntry{
		projectKey: {Name: "ProjectSkill", ClonePath: filepath.Join(projectSkillsDir, projectKey)},
		localKey:   {Name: "LocalSkill", ClonePath: filepath.Join(paths.LocalSkillsDir(projRoot), localKey)},
	}
	if err := skills.SaveRegistry(regPath, reg); err != nil {
		t.Fatal(err)
	}
	if changed, err := skills.SetSkillDisabled("Shared", skills.ScopeGlobal, projHex, projRoot, true); err != nil || !changed {
		t.Fatalf("disable global: %v %v", changed, err)
	}
	if changed, err := skills.SetSkillDisabled("ProjectSkill", skills.ScopeProject, projHex, projRoot, true); err != nil || !changed {
		t.Fatalf("disable project: %v %v", changed, err)
	}
	if changed, err := skills.SetSkillDisabled("LocalSkill", skills.ScopeLocal, projHex, projRoot, true); err != nil || !changed {
		t.Fatalf("disable local: %v %v", changed, err)
	}
	stored, err := skills.LoadRegistry(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Global[globalKey].Disabled || !stored.Projects[projHex][projectKey].Disabled || !stored.Projects[projHex][localKey].Disabled {
		t.Fatalf("disabled state not persisted: %+v", stored)
	}
	refs := skills.OrderedSkillRefs(stored, projHex, projRoot)
	if len(refs) != 0 {
		t.Fatalf("disabled skills remain discoverable: %+v", refs)
	}
	all := skills.OrderedSkillRefsIncludingDisabled(stored, projHex, projRoot)
	if len(all) != 3 {
		t.Fatalf("disabled skills missing from full registry view: %+v", all)
	}
	if changed, err := skills.SetSkillDisabled("ProjectSkill", skills.ScopeProject, projHex, projRoot, false); err != nil || !changed {
		t.Fatalf("enable project: %v %v", changed, err)
	}
	stored, err = skills.LoadRegistry(regPath)
	if err != nil || stored.Projects[projHex][projectKey].Disabled {
		t.Fatalf("enabled state not persisted: %+v err=%v", stored, err)
	}
}
