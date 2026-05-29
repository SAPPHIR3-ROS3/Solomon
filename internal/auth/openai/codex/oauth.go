package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	AccountID    string
	ExpiresAt    time.Time
}

type tokenResponse struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

type tokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func Refresh(ctx context.Context, refreshToken string) (TokenSet, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return TokenSet{}, errors.New("refresh_token is empty")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", ClientID)
	form.Set("refresh_token", refreshToken)
	form.Set("scope", "openid profile email")
	return postToken(ctx, form)
}

func exchangeAuthorizationCode(ctx context.Context, code, verifier string) (TokenSet, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return TokenSet{}, errors.New("authorization code is empty")
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", RedirectURI)
	form.Set("code_verifier", verifier)
	return postToken(ctx, form)
}

func postToken(ctx context.Context, form url.Values) (TokenSet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token request build failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return TokenSet{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token request failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token response read failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return TokenSet{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var te tokenError
		if json.Unmarshal(body, &te) == nil && te.Error != "" {
			if te.ErrorDescription != "" {
				err := fmt.Errorf("%s: %s", te.Error, te.ErrorDescription)
				logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token endpoint error", logging.LogOptions{Params: map[string]any{"status": resp.StatusCode, "err": err.Error()}})
				return TokenSet{}, err
			}
			err := fmt.Errorf("%s", te.Error)
			logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token endpoint error", logging.LogOptions{Params: map[string]any{"status": resp.StatusCode, "err": err.Error()}})
			return TokenSet{}, err
		}
		err := fmt.Errorf("token endpoint: %s: %s", resp.Status, string(body))
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token endpoint error", logging.LogOptions{Params: map[string]any{"status": resp.StatusCode, "err": err.Error()}})
		return TokenSet{}, err
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token response unmarshal failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return TokenSet{}, err
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth token response missing access_token", logging.LogOptions{Params: nil})
		return TokenSet{}, errors.New("token response missing access_token")
	}
	expires := time.Now().Add(time.Hour)
	if tr.ExpiresIn > 0 {
		expires = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	accountID := ChatGPTAccountIDFromJWT(tr.AccessToken)
	if accountID == "" && tr.IDToken != "" {
		accountID = ChatGPTAccountIDFromJWT(tr.IDToken)
	}
	refresh := strings.TrimSpace(tr.RefreshToken)
	if refresh == "" {
		refresh = strings.TrimSpace(form.Get("refresh_token"))
	}
	return TokenSet{
		AccessToken:  tr.AccessToken,
		RefreshToken: refresh,
		AccountID:    accountID,
		ExpiresAt:    expires,
	}, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Start-Process", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if cmd != nil {
		return cmd.Start()
	}
	return nil
}
