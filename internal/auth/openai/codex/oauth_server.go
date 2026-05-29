package codex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

var (
	loginMu      sync.Mutex
	callbackLn   net.Listener
	callbackSrv  *http.Server
	activeWaiter *oauthWaiter
)

type oauthWaiter struct {
	state    string
	verifier string
	codeCh   chan string
	errCh    chan error
}

func ensureCallbackServer() error {
	loginMu.Lock()
	defer loginMu.Unlock()
	if callbackLn != nil {
		return nil
	}
	ln, err := net.Listen("tcp", CallbackAddr)
	if err != nil {
		err = fmt.Errorf("oauth callback listen %s: %w", CallbackAddr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth callback server listen failed", logging.LogOptions{Params: map[string]any{"addr": CallbackAddr, "err": err.Error()}})
		return err
	}
	srv := &http.Server{Handler: http.HandlerFunc(oauthCallbackHandler)}
	callbackLn = ln
	callbackSrv = srv
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != CallbackPath {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		desc := strings.TrimSpace(q.Get("error_description"))
		if desc != "" {
			e = e + ": " + desc
		}
		failOAuthWaiter(fmt.Errorf("oauth error: %s", e))
		writeOAuthHTML(w, "Login failed", e, false)
		return
	}
	gotState := strings.TrimSpace(q.Get("state"))
	code := strings.TrimSpace(q.Get("code"))

	loginMu.Lock()
	wait := activeWaiter
	loginMu.Unlock()

	if wait == nil {
		writeOAuthHTML(w, "No pending sign-in", "Return to Solomon and run /connect to start a new sign-in.", false)
		return
	}
	if gotState != wait.state {
		writeOAuthHTML(w, "Sign-in link expired",
			"This browser tab is from an older /connect attempt. Close it, run /connect again in Solomon, and complete sign-in only in the new browser window.",
			false)
		return
	}
	if code == "" {
		failOAuthWaiter(errors.New("oauth missing code"))
		writeOAuthHTML(w, "Login failed", "missing authorization code", false)
		return
	}
	select {
	case wait.codeCh <- code:
	default:
	}
	writeOAuthHTML(w, "Login successful", "You can close this tab and return to Solomon.", true)
}

func failOAuthWaiter(err error) {
	loginMu.Lock()
	wait := activeWaiter
	loginMu.Unlock()
	if wait == nil {
		return
	}
	select {
	case wait.errCh <- err:
	default:
	}
}

func writeOAuthHTML(w http.ResponseWriter, title, body string, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	color := "#fc533a"
	if ok {
		color = "#10a37f"
	}
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>%s</title></head><body style="font-family:system-ui;background:#131010;color:#f1ecec;padding:2rem"><h1 style="color:%s">%s</h1><p>%s</p></body></html>`, title, color, title, body)
}

func Login(ctx context.Context, out io.Writer) (ts TokenSet, err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "OAuth login failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
	}()
	if err := ensureCallbackServer(); err != nil {
		return TokenSet{}, err
	}
	pkce, err := NewPKCE()
	if err != nil {
		return TokenSet{}, err
	}
	wait := &oauthWaiter{
		state:    pkce.State,
		verifier: pkce.Verifier,
		codeCh:   make(chan string, 1),
		errCh:    make(chan error, 1),
	}
	loginMu.Lock()
	if activeWaiter != nil {
		select {
		case activeWaiter.errCh <- errors.New("oauth sign-in replaced by a new /connect; run /connect again"):
		default:
		}
	}
	activeWaiter = wait
	loginMu.Unlock()
	defer func() {
		loginMu.Lock()
		if activeWaiter == wait {
			activeWaiter = nil
		}
		loginMu.Unlock()
	}()

	authURL := BuildAuthorizeURL(pkce)
	if out != nil {
		fmt.Fprintln(out, "Opening browser for ChatGPT sign-in…")
		fmt.Fprintln(out, "Complete sign-in in the browser window that opens now.")
		fmt.Fprintln(out, "Do not run /connect again until finished or this attempt will fail.")
		fmt.Fprintf(out, "If the browser does not open, paste this URL into a new tab (same attempt only):\n%s\n", authURL)
	}
	_ = openBrowser(authURL)

	select {
	case <-ctx.Done():
		return TokenSet{}, ctx.Err()
	case err := <-wait.errCh:
		return TokenSet{}, err
	case code := <-wait.codeCh:
		return exchangeAuthorizationCode(ctx, code, wait.verifier)
	case <-time.After(10 * time.Minute):
		logging.Log(logging.ERROR_LOG_LEVEL, "OAuth login timed out", logging.LogOptions{Params: map[string]any{"timeout": "10m"}})
		return TokenSet{}, errors.New("oauth login timed out")
	}
}
