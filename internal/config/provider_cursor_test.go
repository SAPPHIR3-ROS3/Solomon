package config

import "testing"

func TestProviderIsCursorAPI(t *testing.T) {
	p := Provider{Name: ProviderNameCursorAPI, AuthKind: AuthKindCursorAPI}
	if !p.IsCursorAPI() {
		t.Fatal("expected cursor api")
	}
	p2 := Provider{Name: "Other", AuthKind: AuthKindAPIKey}
	if p2.IsCursorAPI() {
		t.Fatal("expected false")
	}
}
