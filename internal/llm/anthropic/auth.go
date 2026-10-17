package anthropic

import (
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
)

const APIVersion = claude.APIVersion
const OAuthBetaFastMode = "fast-mode-2026-02-01"
const OAuthBeta = claude.OAuthBeta + "," + OAuthBetaFastMode
const claudeCodeOAuthEntrypoint = "sdk-cli"

type AuthKind int

const (
	AuthAPIKey AuthKind = iota
	AuthOAuthBearer
)

type Auth struct {
	Kind  AuthKind
	Token string
}

func AuthFromAPIKey(token string) Auth {
	return Auth{Kind: AuthAPIKey, Token: strings.TrimSpace(token)}
}
func AuthFromOAuthBearer(token string) Auth {
	return Auth{Kind: AuthOAuthBearer, Token: strings.TrimSpace(token)}
}

// ApplyTo remains available to callers constructing auxiliary requests. SDK
// requests retain the SDK's real Go runtime/version metadata.
func (a Auth) ApplyTo(req *http.Request) {
	req.Header.Set("anthropic-version", APIVersion)
	if a.OAuth() {
		req.Header.Del("x-api-key")
		req.Header.Set("Authorization", "Bearer "+a.Token)
		req.Header.Set("anthropic-beta", OAuthBeta)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("user-agent", "claude-cli/"+claude.Version())
		req.Header.Set("x-app", "cli")
		req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	} else {
		req.Header.Del("Authorization")
		req.Header.Set("x-api-key", a.Token)
	}
}
func (a Auth) ApplyStreamTo(req *http.Request) {
	a.ApplyTo(req)
	if a.OAuth() {
		req.Header.Set("x-stainless-helper-method", "stream")
	}
}
func (a Auth) OAuth() bool { return a.Kind == AuthOAuthBearer }
