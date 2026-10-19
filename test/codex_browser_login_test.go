package test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
)

func TestCodexBrowserLoginRedirectAndPKCE(t *testing.T) {
	requireCodexCallbackPort(t)
	var authorization url.Values
	var redirect string
	var exchanges atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		body := map[string]string{}
		for key := range r.Form {
			body[key] = r.Form.Get(key)
		}
		sum := sha256.Sum256([]byte(body["code_verifier"]))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != authorization.Get("code_challenge") {
			t.Error("PKCE challenge does not match verifier")
		}
		if authorization.Get("state") == body["code_verifier"] {
			t.Error("state must match and be independent of verifier")
		}
		if body["redirect_uri"] != redirect || body["code"] != "single-use-code" || body["client_id"] != codex.ClientID {
			t.Errorf("invalid exchange: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)
	}))
	defer tokenServer.Close()
	restore := codex.SetTokenEndpointForTest(tokenServer.URL)
	defer restore()
	var output strings.Builder
	tokens, err := codex.LoginWithBrowser(context.Background(), &output, func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		authorization = parsed.Query()
		redirect = authorization.Get("redirect_uri")
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
	if !strings.Contains(output.String(), "ChatGPT sign-in complete.") {
		t.Fatal("completion not reported")
	}
	u, _ := url.Parse(redirect)
	conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("callback listener was not closed")
	}
}

func TestCodexBrowserLoginExchangeFailureNeverShowsSuccess(t *testing.T) {
	requireCodexCallbackPort(t)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"secret-must-not-escape"}`)
	}))
	defer tokenServer.Close()
	restore := codex.SetTokenEndpointForTest(tokenServer.URL)
	defer restore()
	_, err := codex.LoginWithBrowser(context.Background(), io.Discard, func(rawURL string) error {
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

func TestCodexBrowserLoginCancellationClosesListener(t *testing.T) {
	requireCodexCallbackPort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var redirect string
	_, err := codex.LoginWithBrowser(ctx, io.Discard, func(rawURL string) error {
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

func TestCodexTokenRefreshFallbackAndNoRedirect(t *testing.T) {
	isolateCodexOAuth(t)
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
		restore := codex.SetTokenEndpointForTest(server.URL)
		tokens, err := codex.Refresh(context.Background(), "existing-refresh")
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

func requireCodexCallbackPort(t *testing.T) {
	t.Helper()
	isolateCodexOAuth(t)
	listener, err := net.Listen("tcp", codex.CallbackAddr)
	if err != nil {
		t.Skipf("existing process owns Codex callback port: %v", err)
	}
	listener.Close()
}

func TestCodexBrowserLoginRetryReleasesPreviousListener(t *testing.T) {
	requireCodexCallbackPort(t)
	t.Setenv("SOLOMON_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`)
	}))
	defer server.Close()
	defer codex.SetTokenEndpointForTest(server.URL)()
	previousRegistry := codex.CodexVersionRegistryURL
	codex.CodexVersionRegistryURL = server.URL
	defer func() { codex.CodexVersionRegistryURL = previousRegistry }()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := codex.LoginWithBrowser(ctx, io.Discard, func(string) error { close(started); return nil })
		finished <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	tokens, err := codex.LoginWithBrowser(ctx, io.Discard, func(raw string) error {
		u, _ := url.Parse(raw)
		q := u.Query()
		response, err := http.Get(q.Get("redirect_uri") + "?code=new-code&state=" + url.QueryEscape(q.Get("state")))
		if err != nil {
			return err
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		return nil
	})
	if err != nil || tokens.AccessToken != "new-access" {
		t.Fatalf("retry tokens=%+v err=%v", tokens, err)
	}
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("previous attempt error=%v", err)
		}
	case <-ctx.Done():
		t.Fatal("previous attempt did not stop")
	}
}

func isolateCodexOAuth(t *testing.T) {
	t.Helper()
	t.Setenv("SOLOMON_HOME", t.TempDir())
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"version":"0.153.3"}`) }))
	t.Cleanup(registry.Close)
	previous := codex.CodexVersionRegistryURL
	codex.CodexVersionRegistryURL = registry.URL
	t.Cleanup(func() { codex.CodexVersionRegistryURL = previous })
}
