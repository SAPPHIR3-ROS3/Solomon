package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
)

func TestRuleActivationPersistsAcrossScopesAndRemoval(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	if _, err := instructions.AddRule(instructions.ScopeGlobal, "", "global rule"); err != nil {
		t.Fatal(err)
	}
	if _, err := instructions.AddRule(instructions.ScopeProject, "project-id", "project rule"); err != nil {
		t.Fatal(err)
	}
	changed, err := instructions.SetRuleEnabled(instructions.ScopeGlobal, "", 1, false)
	if err != nil || !changed {
		t.Fatalf("deactivate global: changed=%v err=%v", changed, err)
	}
	changed, err = instructions.SetRuleEnabled(instructions.ScopeProject, "project-id", 1, false)
	if err != nil || !changed {
		t.Fatalf("deactivate project: changed=%v err=%v", changed, err)
	}
	global, err := instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil || len(global) != 1 || global[0].Enabled {
		t.Fatalf("global state: %+v err=%v", global, err)
	}
	project, err := instructions.ListRules(instructions.ScopeProject, "project-id")
	if err != nil || len(project) != 1 || project[0].Enabled {
		t.Fatalf("project state: %+v err=%v", project, err)
	}
	text, err := instructions.LoadAllRulesText("project-id")
	if err != nil || strings.Contains(text, "global rule") || strings.Contains(text, "project rule") {
		t.Fatalf("inactive rules entered prompt: %q err=%v", text, err)
	}
	var listed strings.Builder
	if err := instructions.WriteRulesList(&listed, "project-id"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed.String(), "[inactive]") {
		t.Fatalf("inactive state missing from list: %q", listed.String())
	}
	if _, err := instructions.AddRule(instructions.ScopeGlobal, "", "new active rule"); err != nil {
		t.Fatal(err)
	}
	if err := instructions.RemoveRule(instructions.ScopeGlobal, "", 2); err != nil {
		t.Fatal(err)
	}
	global, err = instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil || len(global) != 1 || global[0].Number != 1 || global[0].Enabled {
		t.Fatalf("remove/renumber state: %+v err=%v", global, err)
	}
	changed, err = instructions.SetRuleEnabled(instructions.ScopeGlobal, "", 1, true)
	if err != nil || !changed {
		t.Fatalf("activate global: changed=%v err=%v", changed, err)
	}
	global, err = instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil || len(global) != 1 || !global[0].Enabled {
		t.Fatalf("reactivated state: %+v err=%v", global, err)
	}
	globalDir := filepath.Dir(global[0].FilePath)
	if _, err := os.Stat(filepath.Join(globalDir, "rule_01.txt")); err != nil {
		t.Fatal(err)
	}
}
