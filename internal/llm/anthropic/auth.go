package anthropic

import (
	"crypto/rand"
	"encoding/hex"
	goruntime "runtime"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/claudecode"
)

const APIVersion = "2023-06-01"

const (
	OAuthBetaClaudeCode       = "claude-code-20250219"
	OAuthBetaOAuth            = "oauth-2025-04-20"
	OAuthBetaInterleaved      = "interleaved-thinking-2025-05-14"
	OAuthBetaFineGrainedTools = "fine-grained-tool-streaming-2025-05-14"
)

const OAuthBeta = OAuthBetaClaudeCode + "," + OAuthBetaOAuth + "," + OAuthBetaInterleaved + "," + OAuthBetaFineGrainedTools

const anthropicSDKVersion = "0.91.1"

const nodeVersion = "v22.14.0"

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

func stainlessOS() string {
	switch goruntime.GOOS {
	case "darwin":
		return "MacOS"
	case "windows":
		return "Windows"
	default:
		return "Linux"
	}
}

func stainlessArch() string {
	switch goruntime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	case "386":
		return "x32"
	default:
		return "unknown"
	}
}

func randomRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (a Auth) ApplyTo(req *http.Request) {
	req.Header.Set("anthropic-version", APIVersion)
	switch a.Kind {
	case AuthOAuthBearer:
		req.Header.Set("Authorization", "Bearer "+a.Token)
		req.Header.Set("anthropic-beta", OAuthBeta)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("user-agent", "claude-cli/"+claudecode.Version())
		req.Header.Set("x-app", "cli")
		req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
		req.Header.Set("x-stainless-lang", "js")
		req.Header.Set("x-stainless-package-version", anthropicSDKVersion)
		req.Header.Set("x-stainless-os", stainlessOS())
		req.Header.Set("x-stainless-arch", stainlessArch())
		req.Header.Set("x-stainless-runtime", "node")
		req.Header.Set("x-stainless-runtime-version", nodeVersion)
		req.Header.Set("x-stainless-retry-count", "0")
		req.Header.Set("x-claude-code-session-id", randomRequestID())
		req.Header.Set("x-client-request-id", randomRequestID())
	default:
		req.Header.Set("x-api-key", a.Token)
	}
}

func (a Auth) ApplyStreamTo(req *http.Request) {
	a.ApplyTo(req)
	if a.Kind == AuthOAuthBearer {
		req.Header.Set("x-stainless-helper-method", "stream")
	}
}

func (a Auth) OAuth() bool {
	return a.Kind == AuthOAuthBearer
}
