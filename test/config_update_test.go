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
