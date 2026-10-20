package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestTargetedConfigUpdatesPreserveEachOther(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	root.Providers = map[string]*config.Provider{}
	root.Providers["test"] = &config.Provider{Name: "test"}
	root.Current = config.Current{Provider: "test", Model: "old"}
	root.ReasoningEffort = "low"
	if err := config.Save(root); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	if _, err := config.UpdateCurrentModel("test", "new"); err != nil {
		t.Fatalf("update model: %v", err)
	}
	stale := *root
	if _, err := config.UpdateReasoningEffort("high"); err != nil {
		t.Fatalf("update reasoning: %v", err)
	}
	stale.ReasoningEffort = "low"

	saved, err := config.Load()
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if saved.Current.Provider != "test" || saved.Current.Model != "new" {
		t.Fatalf("current model = %s[%s], want new[test]", saved.Current.Model, saved.Current.Provider)
	}
	if saved.ReasoningEffort != "high" {
		t.Fatalf("reasoning effort = %q, want high", saved.ReasoningEffort)
	}
}

func TestModelSwitchRestoresReasoningAndFastMode(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	root.Providers = map[string]*config.Provider{"test": {Name: "test"}}
	root.Current = config.Current{Provider: "test", Model: "alpha"}
	root.ReasoningEffort = "high"
	fast := true
	root.FastMode = &fast
	if err := config.Save(root); err != nil {
		t.Fatalf("save initial config: %v", err)
	}
	if _, err := config.UpdateReasoningEffort("high"); err != nil {
		t.Fatalf("remember alpha reasoning: %v", err)
	}
	off := false
	if _, err := config.UpdateFastMode(true); err != nil {
		t.Fatalf("remember alpha fast: %v", err)
	}
	if _, err := config.UpdateCurrentModel("test", "beta"); err != nil {
		t.Fatalf("switch to beta: %v", err)
	}
	if _, err := config.UpdateReasoningEffort("low"); err != nil {
		t.Fatalf("set beta reasoning: %v", err)
	}
	if _, err := config.UpdateFastMode(off); err != nil {
		t.Fatalf("set beta fast: %v", err)
	}
	restored, err := config.UpdateCurrentModel("test", "alpha")
	if err != nil {
		t.Fatalf("switch back to alpha: %v", err)
	}
	if restored.ReasoningEffort != "high" || !restored.EffectiveFastMode() {
		t.Fatalf("alpha mode = %s fast=%v", restored.ReasoningEffort, restored.EffectiveFastMode())
	}
	back, err := config.UpdateCurrentModel("test", "beta")
	if err != nil {
		t.Fatalf("switch back to beta: %v", err)
	}
	if back.ReasoningEffort != "low" || back.EffectiveFastMode() {
		t.Fatalf("beta mode = %s fast=%v", back.ReasoningEffort, back.EffectiveFastMode())
	}
}
