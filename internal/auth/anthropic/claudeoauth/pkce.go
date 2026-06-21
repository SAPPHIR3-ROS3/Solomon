package claudeoauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
)

const pkceUnreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

type PKCE struct {
	Verifier  string
	Challenge string
}

func NewPKCE() (PKCE, error) {
	ver, err := randomPKCEString(64)
	if err != nil {
		return PKCE{}, err
	}
	sum := sha256.Sum256([]byte(ver))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: ver, Challenge: challenge}, nil
}

func randomPKCEString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := range b {
		out[i] = pkceUnreserved[int(b[i])%len(pkceUnreserved)]
	}
	return string(out), nil
}

func BuildAuthorizeURL(pkce PKCE) string {
	params := url.Values{}
	params.Set("code", "true")
	params.Set("client_id", ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", RedirectURI)
	params.Set("scope", Scopes)
	params.Set("code_challenge", pkce.Challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", pkce.Verifier)
	return fmt.Sprintf("%s?%s", AuthorizeURL, params.Encode())
}
