package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
)

func TestCursorOrderPicksGrok45OverOlderGrok(t *testing.T) {
	ids := []string{
		"grok-4",
		"grok-4-3",
		"grok-4-5",
		"grok-4.5",
		"grok-code",
	}
	flagship := cursorint.FilterModelIDs(ids)
	var grok string
	for _, id := range flagship {
		if strings.Contains(strings.ToLower(id), "grok") {
			grok = id
			break
		}
	}
	if grok != "grok-4.5" && grok != "grok-4-5" {
		t.Fatalf("flagship grok=%q, want grok-4.5 or grok-4-5 in %v", grok, flagship)
	}
	ordered := cursorint.OrderModelIDs(ids)
	if len(ordered) == 0 {
		t.Fatal("empty ordered ids")
	}
	first := strings.ToLower(ordered[0])
	if first != "grok-4.5" && first != "grok-4-5" {
		t.Fatalf("ordered first=%q, want grok 4.5, got %v", ordered[0], ordered)
	}
}

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
	cfg.Providers[config.ProviderNameCursorAPI].AuthKind = config.AuthKindOAuthCursor
	cfg.Providers[config.ProviderNameCursorAPI].OAuthAccessToken = "session-token"
	if config.CursorAPIConfigured(cfg) {
		t.Fatal("OAuth login belongs to Cursor Sub, not Cursor API")
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

func TestFastModeSupportedByProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider config.Provider
		want     bool
	}{
		{
			name: "ChatGPT subscription",
			provider: config.Provider{
				Name:     config.ProviderNameChatGPTSub,
				AuthKind: config.AuthKindOAuthChatGPT,
			},
			want: true,
		},
		{
			name: "Cursor API",
			provider: config.Provider{
				Name:     config.ProviderNameCursorAPI,
				AuthKind: config.AuthKindCursorAPI,
			},
			want: true,
		},
		{
			name: "OpenAI compatible API",
			provider: config.Provider{
				Name:     "OpenAI",
				AuthKind: config.AuthKindAPIKey,
			},
			want: false,
		},
		{
			name: "Claude subscription",
			provider: config.Provider{
				Name:     config.ProviderNameClaudeSub,
				AuthKind: config.AuthKindOAuthClaude,
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := config.FastModeSupportedByProvider(&test.provider); got != test.want {
				t.Fatalf("FastModeSupportedByProvider(%q) = %t, want %t", test.provider.Name, got, test.want)
			}
		})
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
	cfg.FastMode = nil
	chatGPT := &config.Provider{Name: config.ProviderNameChatGPTSub, AuthKind: config.AuthKindOAuthChatGPT}
	if got := cfg.ModelDisplayName(chatGPT, "gpt-5.6"); got != "gpt-5.6 (high) (fast)" {
		t.Fatalf("ChatGPT subscription display=%q", got)
	}
}

func TestMigrateMisnamedCursorSub(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	cfgPath := filepath.Join(home, "config.toml")
	raw := `
[current]
provider = "Cursor API"
model = "composer-2.5"

[providers."Cursor API"]
name = "Cursor API"
auth_kind = "oauth_cursor"
oauth_access_token = "session-jwt.example.token"
oauth_refresh_token = "refresh-token"
`
	if err := os.WriteFile(cfgPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers[config.ProviderNameCursorAPI] != nil {
		t.Fatal("expected Cursor API removed when login was saved under that name")
	}
	sub := cfg.Providers[config.ProviderNameCursorSub]
	if sub == nil || sub.OAuthAccessToken != "session-jwt.example.token" {
		t.Fatalf("expected Cursor Sub with oauth tokens, got %#v", sub)
	}
	if cfg.Current.Provider != config.ProviderNameCursorSub {
		t.Fatalf("current provider=%q", cfg.Current.Provider)
	}
	if strings.Contains(sub.BaseURL, "127.0.0.1") || strings.Contains(sub.BaseURL, "8766") {
		t.Fatalf("Cursor Sub still points at sidecar: %s", sub.BaseURL)
	}
}

func TestEnsureCursorSubRewritesSidecarBaseURL(t *testing.T) {
	p := &config.Provider{
		Name:     config.ProviderNameCursorSub,
		AuthKind: config.AuthKindOAuthCursor,
		BaseURL:  "http://127.0.0.1:8766/v1/",
	}
	config.EnsureCursorSubBaseURL(p)
	if p.BaseURL != config.CursorSubChatBase() {
		t.Fatalf("base=%q", p.BaseURL)
	}
	if strings.Contains(p.BaseURL, "8766") {
		t.Fatalf("sidecar url leaked: %s", p.BaseURL)
	}
}

func TestDropCursorAPIWhenSubPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	cfgPath := filepath.Join(home, "config.toml")
	raw := `
[providers."Cursor API"]
name = "Cursor API"
auth_kind = "cursor_api"
api_key = "crsr_leftover"

[providers."Cursor Sub"]
name = "Cursor Sub"
auth_kind = "oauth_cursor"
oauth_access_token = "session-jwt.example.token"
`
	if err := os.WriteFile(cfgPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers[config.ProviderNameCursorAPI] != nil {
		t.Fatal("expected leftover Cursor API removed when Cursor Sub exists")
	}
	if cfg.Providers[config.ProviderNameCursorSub] == nil {
		t.Fatal("expected Cursor Sub to remain")
	}
}
