package anthropic

import (
	"net/http"
	"strings"
)

const APIVersion = "2023-06-01"

const OAuthBeta = "oauth-2025-04-20"

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

func (a Auth) ApplyTo(req *http.Request) {
	req.Header.Set("anthropic-version", APIVersion)
	switch a.Kind {
	case AuthOAuthBearer:
		req.Header.Set("Authorization", "Bearer "+a.Token)
		req.Header.Set("anthropic-beta", OAuthBeta)
	default:
		req.Header.Set("x-api-key", a.Token)
	}
}
