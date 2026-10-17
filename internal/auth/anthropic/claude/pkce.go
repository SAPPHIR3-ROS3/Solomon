package claude

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
)

type PKCE struct {
	Verifier  string
	Challenge string
	State     string
}

func NewPKCE() (PKCE, error) {
	ver, err := randomPKCEString(64)
	if err != nil {
		return PKCE{}, err
	}
	state, err := randomPKCEString(32)
	if err != nil {
		return PKCE{}, err
	}
	sum := sha256.Sum256([]byte(ver))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: ver, Challenge: challenge, State: state}, nil
}

func randomPKCEString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func BuildAuthorizeURL(pkce PKCE) string {
	return buildAuthorizeURL(pkce, RedirectURI)
}

func buildAuthorizeURL(pkce PKCE, redirect string) string {
	params := url.Values{}
	params.Set("code", "true")
	params.Set("client_id", ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirect)
	params.Set("scope", Scopes)
	params.Set("code_challenge", pkce.Challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", pkce.State)
	return fmt.Sprintf("%s?%s", AuthorizeURL, params.Encode())
}
