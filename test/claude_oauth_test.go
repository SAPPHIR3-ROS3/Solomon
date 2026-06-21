package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claudeoauth"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestClaudeOAuthBuildAuthorizeURL(t *testing.T) {
	pkce, err := claudeoauth.NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	u := claudeoauth.BuildAuthorizeURL(pkce)
	for _, want := range []string{
		"https://claude.ai/oauth/authorize?",
		"code=true",
		"client_id=9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		"redirect_uri=http%3A%2F%2Flocalhost%3A53692%2Fcallback",
		"code_challenge_method=S256",
		"state=" + pkce.Verifier,
	} {
		if !strings.Contains(u, want) {
			t.Fatalf("authorize URL missing %q: %s", want, u)
		}
	}
}

func TestNewClaudeSubProvider(t *testing.T) {
	p, err := config.NewClaudeSubProvider(claudeoauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsClaudeSub() {
		t.Fatal("expected Claude Sub provider")
	}
	if p.OAuthAccessToken != "at" {
		t.Fatalf("access token = %q", p.OAuthAccessToken)
	}
}
