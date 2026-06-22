package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

var tokenEndpoint = TokenURL

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

func exchangeAuthorizationCode(ctx context.Context, code, state, verifier string) (TokenSet, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return TokenSet{}, errors.New("authorization code is empty")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		state = verifier
	}
	return postToken(ctx, map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     ClientID,
		"code":          code,
		"state":         state,
		"redirect_uri": RedirectURI,
		"code_verifier": verifier,
	})
}

func postToken(ctx context.Context, body map[string]string) (TokenSet, error) {
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "Claude OAuth token request failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenSet{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var tr tokenResponse
		if json.Unmarshal(respBody, &tr) == nil && tr.Error != "" {
			msg := tr.Error
			if tr.ErrorDesc != "" {
				msg = msg + ": " + tr.ErrorDesc
			}
			err := fmt.Errorf("%s", msg)
			logging.Log(logging.ERROR_LOG_LEVEL, "Claude OAuth token endpoint error", logging.LogOptions{Params: map[string]any{"status": resp.StatusCode, "err": err.Error()}})
			return TokenSet{}, err
		}
		err := fmt.Errorf("token endpoint: %s: %s", resp.Status, string(respBody))
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
	expires = expires.Add(-5 * time.Minute)
	refresh := strings.TrimSpace(tr.RefreshToken)
	if refresh == "" {
		refresh = strings.TrimSpace(body["refresh_token"])
	}
	return TokenSet{
		AccessToken:  tr.AccessToken,
		RefreshToken: refresh,
		ExpiresAt:    expires,
	}, nil
}
