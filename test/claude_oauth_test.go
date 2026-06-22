package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestClaudeOAuthBuildAuthorizeURL(t *testing.T) {
	pkce, err := claude.NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	u := claude.BuildAuthorizeURL(pkce)
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
	p, err := config.NewClaudeSubProvider(claude.TokenSet{
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

func TestClaudeOAuthRefreshOmitsScope(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at",
			"refresh_token": "rt2",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	restore := claude.SetTokenEndpointForTest(srv.URL)
	defer restore()

	_, err := claude.Refresh(context.Background(), "rt")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := gotBody["scope"]; ok {
		t.Fatalf("refresh must not send scope, got %#v", gotBody)
	}
	if gotBody["grant_type"] != "refresh_token" {
		t.Fatalf("grant_type = %q", gotBody["grant_type"])
	}
}
