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
