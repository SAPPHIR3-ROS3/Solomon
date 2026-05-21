package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
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
	empty := config.Provider{BaseURL: "https://api.openai.com/v1/", AuthKind: config.AuthKindOAuthChatGPT}
	if config.ProviderCredentialsReady(&empty) {
		t.Fatal("oauth without tokens should not be ready")
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
