package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func TestProviderIsCursorAPI(t *testing.T) {
	p := config.Provider{Name: config.ProviderNameCursorAPI, AuthKind: config.AuthKindCursorAPI}
	if !p.IsCursorAPI() {
		t.Fatal("expected cursor api")
	}
	p2 := config.Provider{Name: "Other", AuthKind: config.AuthKindAPIKey}
	if p2.IsCursorAPI() {
		t.Fatal("expected false")
	}
}

func TestCursorFastModeDisplayDefaultAndDisabled(t *testing.T) {
	cfg := &config.Root{ReasoningEffort: "high"}
	p := &config.Provider{Name: config.ProviderNameCursorAPI, AuthKind: config.AuthKindCursorAPI}
	if got := cfg.ModelDisplayName(p, "composer-2.5"); got != "composer-2.5 (high) (fast)" {
		t.Fatalf("default display=%q", got)
	}
	off := false
	cfg.FastMode = &off
	if got := cfg.ModelDisplayName(p, "composer-2.5"); got != "composer-2.5 (high)" {
		t.Fatalf("disabled display=%q", got)
	}
	if got := cfg.ModelDisplayName(&config.Provider{Name: "OpenAI"}, "gpt-5"); got != "gpt-5 (high)" {
		t.Fatalf("non-cursor display=%q", got)
	}
}
