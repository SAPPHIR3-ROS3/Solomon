package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
)

func TestCursorModelOrderOpusAboveSonnet(t *testing.T) {
	ids := []string{
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
		"claude-sonnet-4-6",
		"claude-opus-4-5",
	}
	ordered := cursorint.OrderModelIDs(ids)
	firstSonnet := -1
	firstOpus := -1
	for i, id := range ordered {
		m := strings.ToLower(id)
		if firstSonnet < 0 && strings.Contains(m, "sonnet") {
			firstSonnet = i
		}
		if firstOpus < 0 && strings.Contains(m, "opus") {
			firstOpus = i
		}
	}
	if firstOpus < 0 || firstSonnet < 0 {
		t.Fatalf("missing opus/sonnet in %v", ordered)
	}
	if firstOpus > firstSonnet {
		t.Fatalf("opus should sort above sonnet, got %v", ordered)
	}
	flagship := cursorint.FilterModelIDs(ids)
	for _, id := range flagship {
		if strings.Contains(strings.ToLower(id), "sonnet") && !strings.Contains(strings.ToLower(id), "opus") {
			for _, other := range ids {
				if strings.Contains(strings.ToLower(other), "opus") {
					t.Fatalf("filter picked sonnet %q over opus candidates %v", id, ids)
				}
			}
		}
	}
}

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

func TestCursorAPIConfigured(t *testing.T) {
	if config.CursorAPIConfigured(nil) {
		t.Fatal("nil cfg")
	}
	cfg := &config.Root{Providers: map[string]*config.Provider{"p": {Name: "p", BaseURL: "http://x", APIKey: "k"}}}
	if config.CursorAPIConfigured(cfg) {
		t.Fatal("expected false without Cursor API provider")
	}
	cfg.Providers[config.ProviderNameCursorAPI] = &config.Provider{
		Name:     config.ProviderNameCursorAPI,
		AuthKind: config.AuthKindCursorAPI,
		BaseURL:  cursorint.DefaultBaseURL(cursorint.DefaultPort),
		APIKey:   "cursor-key",
	}
	if !config.CursorAPIConfigured(cfg) {
		t.Fatal("expected true with Cursor API key")
	}
	cfg.Providers[config.ProviderNameCursorAPI].APIKey = ""
	if config.CursorAPIConfigured(cfg) {
		t.Fatal("expected false without API key")
	}
}

func TestCursorModelsFallbackWhenProxyHasNoModelsEndpoint(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	p := &config.Provider{
		Name:     "Cursor test",
		AuthKind: config.AuthKindCursorAPI,
		BaseURL:  srv.URL + "/v1/",
		APIKey:   "cursor-key",
	}
	cfg := &config.Root{}
	got, err := connect.ListModelsForProvider(context.Background(), cfg, p)
	if err != nil {
		t.Fatal(err)
	}
	want := cursorint.DefaultModelIDs()
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("models=%v, want %v", got, want)
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
