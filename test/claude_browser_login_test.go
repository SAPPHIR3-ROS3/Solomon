package test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
)

func TestBrowserLoginRedirectAndPKCE(t *testing.T) {
	// Force the preferred port to be unavailable; existing listeners are untouched.
	occupied, _ := net.Listen("tcp", claude.CallbackAddr)
	if occupied != nil {
		defer occupied.Close()
	}
	var authorization url.Values
	var redirect string
	var exchanges atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges.Add(1)
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		sum := sha256.Sum256([]byte(body["code_verifier"]))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != authorization.Get("code_challenge") {
			t.Error("PKCE challenge does not match verifier")
		}
		if body["state"] != authorization.Get("state") || body["state"] == body["code_verifier"] {
			t.Error("state must match and be independent of verifier")
		}
		if body["redirect_uri"] != redirect || body["code"] != "single-use-code" || body["client_id"] != claude.ClientID {
			t.Errorf("invalid exchange: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)
	}))
	defer tokenServer.Close()
	restore := claude.SetTokenEndpointForTest(tokenServer.URL)
	defer restore()
	var output strings.Builder
	tokens, err := claude.LoginWithBrowser(context.Background(), &output, func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		authorization = parsed.Query()
		redirect = authorization.Get("redirect_uri")
		if occupied != nil && redirect == claude.RedirectURI {
			t.Error("did not select an available port")
		}
		for _, query := range []string{"?error=access_denied&state=unrelated", "?code=old-code&state=unrelated", "?state=" + url.QueryEscape(authorization.Get("state"))} {
			resp, err := http.Get(redirect + query)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("unrelated callback status: %d", resp.StatusCode)
			}
		}
		callback := redirect + "?code=single-use-code&state=" + url.QueryEscape(authorization.Get("state"))
		resp, err := http.Get(callback)
		if err != nil {
			return err
		}
		page, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(page), "Login successful") {
			t.Errorf("callback: %d %s", resp.StatusCode, page)
		}
		duplicate, err := http.Get(callback)
		if err != nil {
			return err
		}
		duplicate.Body.Close()
		if duplicate.StatusCode != http.StatusConflict {
			t.Error("authorization code accepted twice")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh" || exchanges.Load() != 1 {
		t.Fatalf("tokens=%+v exchanges=%d", tokens, exchanges.Load())
	}
	if time.Until(tokens.ExpiresAt) < 59*time.Minute {
		t.Fatal("expiry safety margin applied twice")
	}
	if !strings.Contains(output.String(), "Claude sign-in complete.") {
		t.Fatal("completion not reported")
	}
	u, _ := url.Parse(redirect)
	conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("callback listener was not closed")
	}
}

func TestBrowserLoginExchangeFailureNeverShowsSuccess(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"secret-must-not-escape"}`)
	}))
	defer tokenServer.Close()
	restore := claude.SetTokenEndpointForTest(tokenServer.URL)
	defer restore()
	_, err := claude.LoginWithBrowser(context.Background(), io.Discard, func(rawURL string) error {
		u, _ := url.Parse(rawURL)
		q := u.Query()
		resp, err := http.Get(q.Get("redirect_uri") + "?code=expired&state=" + url.QueryEscape(q.Get("state")))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		page, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusBadRequest || strings.Contains(string(page), "Login successful") {
			t.Error("failed exchange reported success")
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") || strings.Contains(err.Error(), "secret-must-not-escape") {
		t.Fatalf("error=%v", err)
	}
}

func TestBrowserLoginCancellationClosesListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var redirect string
	_, err := claude.LoginWithBrowser(ctx, io.Discard, func(rawURL string) error {
		u, _ := url.Parse(rawURL)
		redirect = u.Query().Get("redirect_uri")
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	u, _ := url.Parse(redirect)
	conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("canceled listener still open")
	}
}

func TestTokenRefreshFallbackAndNoRedirect(t *testing.T) {
	var leaked atomic.Bool
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true) }))
	defer sink.Close()
	for _, redirect := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if redirect {
				w.Header().Set("Location", sink.URL)
				w.WriteHeader(http.StatusTemporaryRedirect)
				return
			}
			io.WriteString(w, `{"access_token":"new-access","expires_in":3600}`)
		}))
		restore := claude.SetTokenEndpointForTest(server.URL)
		tokens, err := claude.Refresh(context.Background(), "existing-refresh")
		restore()
		server.Close()
		if redirect && err == nil {
			t.Fatal("accepted token endpoint redirect")
		}
		if !redirect && (err != nil || tokens.RefreshToken != "existing-refresh") {
			t.Fatalf("refresh=%+v error=%v", tokens, err)
		}
	}
	if leaked.Load() {
		t.Fatal("token body followed redirect")
	}
}
