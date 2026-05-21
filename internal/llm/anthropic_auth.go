package llm

import (
	"net/http"
	"strings"
)

const AnthropicOAuthBeta = "oauth-2025-04-20"

type AnthropicAuthKind int

const (
	AnthropicAuthAPIKey AnthropicAuthKind = iota
	AnthropicAuthOAuthBearer
)

type AnthropicAuth struct {
	Kind  AnthropicAuthKind
	Token string
}

func AnthropicAuthFromAPIKey(token string) AnthropicAuth {
	return AnthropicAuth{Kind: AnthropicAuthAPIKey, Token: strings.TrimSpace(token)}
}

func AnthropicAuthFromOAuthBearer(token string) AnthropicAuth {
	return AnthropicAuth{Kind: AnthropicAuthOAuthBearer, Token: strings.TrimSpace(token)}
}

func (a AnthropicAuth) ApplyTo(req *http.Request) {
	req.Header.Set("anthropic-version", AnthropicAPIVersion)
	switch a.Kind {
	case AnthropicAuthOAuthBearer:
		req.Header.Set("Authorization", "Bearer "+a.Token)
		req.Header.Set("anthropic-beta", AnthropicOAuthBeta)
	default:
		req.Header.Set("x-api-key", a.Token)
	}
}
