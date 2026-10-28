package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

var tokenEndpoint = TokenURL

// Token POSTs must never follow redirects to another origin.
var tokenClient = &http.Client{Transport: networkhttp.NewDefaultTransport(), Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func Refresh(ctx context.Context, refreshToken string) (TokenSet, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return TokenSet{}, errors.New("refresh_token is empty")
	}
	return postToken(ctx, map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     ClientID,
		"refresh_token": refreshToken,
	})
}

func exchangeCode(ctx context.Context, code, state, verifier, redirect string) (TokenSet, error) {
	return postToken(ctx, map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     ClientID,
		"code":          code,
		"state":         state,
		"redirect_uri":  redirect,
		"code_verifier": verifier,
	})
}

func postToken(ctx context.Context, body map[string]string) (TokenSet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := json.Marshal(body)
	if err != nil {
		return TokenSet{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewReader(raw))
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "Claude OAuth token request build failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return TokenSet{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := tokenClient.Do(req)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "Claude OAuth token request failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return TokenSet{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var tr tokenResponse
		if json.Unmarshal(respBody, &tr) == nil && tr.Error != "" {
			msg := tr.Error
			// Do not echo arbitrary response bodies or token material.
			switch msg {
			case "invalid_grant", "invalid_token":
				msg += "; authorization expired or was already used; run /connect"
			default:
				msg = "Claude OAuth token request rejected"
			}
			err := fmt.Errorf("%s", msg)
			logging.Log(logging.ERROR_LOG_LEVEL, "Claude OAuth token endpoint error", logging.LogOptions{Params: map[string]any{"status": resp.StatusCode, "err": err.Error()}})
			return TokenSet{}, err
		}
		err := fmt.Errorf("Claude OAuth token endpoint: HTTP %d; start sign-in again if authorization has expired", resp.StatusCode)
		logging.Log(logging.ERROR_LOG_LEVEL, "Claude OAuth token endpoint error", logging.LogOptions{Params: map[string]any{"status": resp.StatusCode, "err": err.Error()}})
		return TokenSet{}, err
	}
	var tr tokenResponse
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return TokenSet{}, err
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		return TokenSet{}, errors.New("token response missing access_token")
	}
	expires := time.Now().Add(time.Hour)
	if tr.ExpiresIn > 0 {
		expires = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	// Config refreshes proactively; store the actual expiry once, without
	// subtracting a second safety margin here.
	refresh := strings.TrimSpace(tr.RefreshToken)
	if refresh == "" {
		refresh = strings.TrimSpace(body["refresh_token"])
	}
	if refresh == "" {
		return TokenSet{}, errors.New("Claude OAuth response missing refresh_token; run /connect")
	}
	return TokenSet{
		AccessToken:  strings.TrimSpace(tr.AccessToken),
		RefreshToken: refresh,
		ExpiresAt:    expires,
	}, nil
}

func SetTokenEndpointForTest(url string) func() {
	old := tokenEndpoint
	tokenEndpoint = url
	return func() { tokenEndpoint = old }
}
