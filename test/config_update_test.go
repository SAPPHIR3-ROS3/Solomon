package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestInstallLayoutSurvivesConfigRoundTrip(t *testing.T) {
	for _, layout := range []string{"split", "standalone"} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%v", layout, legacy), func(t *testing.T) {
				t.Setenv("SOLOMON_HOME", t.TempDir())
				text := fmt.Sprintf("install_layout = %q\n", layout)
				if legacy {
					text += "[[providers]]\nname = 'fixture'\n"
				}
				if err := os.WriteFile(filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml"), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				root, err := config.Load()
				if err != nil || root.EffectiveInstallLayout() != layout {
					t.Fatalf("read layout: %+v, %v", root, err)
				}
				if err := config.Save(root); err != nil {
					t.Fatal(err)
				}
				reloaded, err := config.Load()
				if err != nil || reloaded.EffectiveInstallLayout() != layout {
					t.Fatalf("saved layout: %+v, %v", reloaded, err)
				}
			})
		}
	}
}

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

func TestConfigModeUpdatesSkipLiveRoleValidation(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	previous := config.RolesModelLister
	calls := 0
	config.RolesModelLister = func(context.Context, *config.Root, *config.Provider) ([]string, error) {
		calls++
		return nil, fmt.Errorf("provider unavailable")
	}
	t.Cleanup(func() { config.RolesModelLister = previous })
	root := config.EmptyRoot()
	root.Providers = map[string]*config.Provider{"test": {Name: "test"}}
	root.Current = config.Current{Provider: "test", Model: "alpha"}
	root.ReasoningEffort = "high"
	root.Roles = config.Roles{
		Table:    config.RolesTable{Characteristics: []string{"reasoning"}},
		Subagent: []config.SubagentRoleConfig{{Provider: "test", Model: "worker"}},
	}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	if _, err := config.UpdateCurrentModel("test", "beta"); err != nil {
		t.Fatal(err)
	}
	if _, err := config.UpdateReasoningEffort("low"); err != nil {
		t.Fatal(err)
	}
	if _, err := config.UpdateFastMode(false); err != nil {
		t.Fatal(err)
	}
	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Current.Model != "beta" || saved.ReasoningEffort != "low" || saved.EffectiveFastMode() {
		t.Fatalf("preferences did not persist: model=%s reasoning=%s fast=%v", saved.Current.Model, saved.ReasoningEffort, saved.EffectiveFastMode())
	}
	if calls != 0 {
		t.Fatalf("routine persistence made %d live model requests", calls)
	}
	if err := config.ValidateRoles(context.Background(), saved); err == nil || calls != 1 {
		t.Fatalf("explicit role validation must still check the provider: err=%v calls=%d", err, calls)
	}
}
