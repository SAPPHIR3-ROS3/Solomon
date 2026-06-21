package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestModelPassesChatGPTSubFilter(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"gpt-4o", true},
		{"gpt-4.1-mini", true},
		{"GPT-5", true},
		{"gpt-image-1", false},
		{"gpt-realtime-preview", false},
		{"gpt-audio-1", false},
		{"sora-2", false},
		{"o3-mini", false},
		{"", false},
	}
	for _, tc := range tests {
		got := config.ModelPassesChatGPTSubFilter(tc.id)
		if got != tc.want {
			t.Errorf("ModelPassesChatGPTSubFilter(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestModelPassesChatGPTSubPickerFilter(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"gpt-5.4", true},
		{"gpt-5.4-mini", true},
		{"gpt-5-pro", false},
		{"gpt-5.4-pro", false},
		{"gpt-image-1", false},
	}
	for _, tc := range tests {
		got := config.ModelPassesChatGPTSubPickerFilter(tc.id)
		if got != tc.want {
			t.Errorf("ModelPassesChatGPTSubPickerFilter(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestProviderCredentialsReady(t *testing.T) {
	api := config.Provider{BaseURL: "https://api.openai.com/v1/", APIKey: "sk-x", AuthKind: config.AuthKindAPIKey}
	if !config.ProviderCredentialsReady(&api) {
		t.Fatal("api key provider should be ready")
	}
	oauth := config.Provider{
		BaseURL:           "https://api.openai.com/v1/",
		AuthKind:          config.AuthKindOAuthChatGPT,
		OAuthRefreshToken: "rt",
	}
	if !config.ProviderCredentialsReady(&oauth) {
		t.Fatal("oauth provider with refresh should be ready")
	}
	claudeOAuth := config.Provider{
		BaseURL:           "https://api.anthropic.com",
		AuthKind:          config.AuthKindOAuthClaude,
		APIProtocol:       config.APIProtocolAnthropic,
		OAuthAccessToken:  "at",
	}
	if !config.ProviderCredentialsReady(&claudeOAuth) {
		t.Fatal("claude oauth provider with access token should be ready")
	}
	empty := config.Provider{BaseURL: "https://api.openai.com/v1/", AuthKind: config.AuthKindOAuthChatGPT}
	if config.ProviderCredentialsReady(&empty) {
		t.Fatal("oauth without tokens should not be ready")
	}
}

func TestModelPassesClaudeSubFilter(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"claude-sonnet-4-20250514", true},
		{"claude-3-5-haiku-20241022", true},
		{"gpt-4o", false},
		{"", false},
	}
	for _, tc := range tests {
		got := config.ModelPassesClaudeSubFilter(tc.id)
		if got != tc.want {
			t.Errorf("ModelPassesClaudeSubFilter(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestIsClaudeSub(t *testing.T) {
	p := config.Provider{Name: config.ProviderNameClaudeSub, AuthKind: config.AuthKindOAuthClaude, APIProtocol: config.APIProtocolAnthropic}
	if !p.IsClaudeSub() {
		t.Fatal("expected Claude Sub provider")
	}
	if !p.UsesAnthropicOAuthBearer() {
		t.Fatal("expected anthropic oauth bearer")
	}
	wrongName := config.Provider{Name: "Claude", AuthKind: config.AuthKindOAuthClaude}
	if wrongName.IsClaudeSub() {
		t.Fatal("wrong name should not be Claude Sub")
	}
}

func TestIsAnthropicClaudeCodeOAuthToken(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"sk-ant-oat01-abc", true},
		{"  sk-ant-oat-xyz", true},
		{"sk-ant-api03-key", false},
		{"sk-x", false},
		{"", false},
	}
	for _, tc := range tests {
		got := config.IsAnthropicClaudeCodeOAuthToken(tc.token)
		if got != tc.want {
			t.Errorf("IsAnthropicClaudeCodeOAuthToken(%q) = %v, want %v", tc.token, got, tc.want)
		}
	}
}

func TestUsesAnthropicOAuthBearer_ClaudeCodeTokenInAPIKey(t *testing.T) {
	p := config.Provider{
		BaseURL:     "https://api.anthropic.com",
		APIKey:      "sk-ant-oat01-test",
		APIProtocol: config.APIProtocolAnthropic,
		AuthKind:    config.AuthKindAPIKey,
	}
	if !p.UsesAnthropicOAuthBearer() {
		t.Fatal("expected oauth bearer for Claude Code token in api_key field")
	}
}

func TestResolveProviderBearer_ClaudeSub(t *testing.T) {
	p := config.Provider{
		Name:             config.ProviderNameClaudeSub,
		BaseURL:          "https://api.anthropic.com",
		AuthKind:         config.AuthKindOAuthClaude,
		APIProtocol:      config.APIProtocolAnthropic,
		OAuthAccessToken: "sk-ant-oat-test",
		OAuthExpiresAt:   "2099-01-01T00:00:00Z",
	}
	bearer, err := config.ResolveProviderBearer(t.Context(), nil, &p)
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "sk-ant-oat-test" {
		t.Fatalf("bearer = %q", bearer)
	}
}

func TestAppendOrUpdateProvider(t *testing.T) {
	r := config.EmptyRoot()
	p1 := config.Provider{Name: "a", BaseURL: "https://x/v1/", APIKey: "k1"}
	config.AppendOrUpdateProvider(r, p1)
	if len(r.Providers) != 1 || r.Providers["a"] == nil || r.Providers["a"].APIKey != "k1" {
		t.Fatal("expected one provider")
	}
	p2 := config.Provider{Name: "a", BaseURL: "https://y/v1/", APIKey: "k2"}
	config.AppendOrUpdateProvider(r, p2)
	if len(r.Providers) != 1 || r.Providers["a"].APIKey != "k2" {
		t.Fatal("expected upsert not append")
	}
}
