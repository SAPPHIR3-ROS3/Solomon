package test

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"testing"
)

func TestModelReasoningUpdatesPreserveOtherModelAndFastMode(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	root.Providers = map[string]*config.Provider{"test": {Name: "test"}}
	root.Current = config.Current{Provider: "test", Model: "alpha"}
	root.ReasoningEffort = "low"
	fast := true
	root.ModelModes = map[string]map[string]config.ModelMode{"test": {"beta": {ReasoningEffort: "medium", FastMode: &fast}}}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	saved, err := config.UpdateModelReasoning("test", "beta", "high")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Current.Model != "alpha" || saved.ReasoningEffort != "low" {
		t.Fatal("updating another model changed the active model")
	}
	beta := saved.ModelModes["test"]["beta"]
	if beta.ReasoningEffort != "high" || beta.FastMode == nil || !*beta.FastMode {
		t.Fatal("model preference or fast mode lost")
	}
	if err := config.AssignCurrentModel(saved, "test", "beta"); err != nil {
		t.Fatal(err)
	}
	if saved.ReasoningEffort != "high" {
		t.Fatal("model reasoning was not restored")
	}
	if _, err = config.UpdateModelReasoning("test", "beta", ""); err != nil {
		t.Fatal(err)
	}
	latest, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err = config.AssignCurrentModel(latest, "test", "beta"); err != nil {
		t.Fatal(err)
	}
	if latest.ReasoningEffort != "" {
		t.Fatal("provider default inherited the outgoing model effort")
	}
}
