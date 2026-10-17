package claude

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
)

type loginResult struct {
	tokens TokenSet
	err    error
}

// Login keeps the browser -> authorization -> localhost redirect experience.
// Each attempt owns its listener and state; canceling one cannot affect another.
func Login(ctx context.Context, out io.Writer) (TokenSet, error) {
	return LoginWithBrowser(ctx, out, openBrowser)
}

// LoginWithBrowser allows desktop hosts to supply their own browser launcher.
func LoginWithBrowser(ctx context.Context, out io.Writer, browser func(string) error) (TokenSet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if browser == nil {
		browser = openBrowser
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return TokenSet{}, err
	}
	pkce, err := NewPKCE()
	if err != nil {
		return TokenSet{}, err
	}
	ln, err := net.Listen("tcp", CallbackAddr)
	if err != nil {
		ln, err = net.Listen("tcp", net.JoinHostPort(CallbackHost, "0"))
	}
	if err != nil {
		return TokenSet{}, fmt.Errorf("Claude sign-in: cannot listen for browser redirect: %w", err)
	}
	redirect := fmt.Sprintf("http://localhost:%d%s", ln.Addr().(*net.TCPAddr).Port, CallbackPath)
	results := make(chan loginResult, 1)
	srv := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	srv.Handler = callbackHandler(ctx, pkce, redirect, results)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case results <- loginResult{err: err}:
			default:
			}
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		if srv.Shutdown(shutdownCtx) != nil {
			_ = srv.Close()
		}
	}()
	authURL := buildAuthorizeURL(pkce, redirect)
	if out != nil {
		fmt.Fprintln(out, "Opening browser for Claude sign-in…")
		fmt.Fprintln(out, "Complete sign-in in the browser; you will return to Solomon automatically.")
		fmt.Fprintf(out, "If the browser does not open, paste this URL into a new tab:\n%s\n", authURL)
	}
	if err := browser(authURL); err != nil && out != nil {
		fmt.Fprintln(out, "Could not open the browser automatically. Use the sign-in URL above.")
	}
	select {
	case <-ctx.Done():
		return TokenSet{}, fmt.Errorf("Claude sign-in: %w", ctx.Err())
	case result := <-results:
		if result.err != nil {
			return TokenSet{}, result.err
		}
		if out != nil {
			fmt.Fprintln(out, "Claude sign-in complete.")
		}
		return result.tokens, nil
	}
}

func callbackHandler(ctx context.Context, pkce PKCE, redirect string, results chan<- loginResult) http.Handler {
	var mu sync.Mutex
	consumed := false
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != CallbackPath {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		// Validate state before processing errors too: unrelated tabs cannot cancel a login.
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(pkce.State)) != 1 {
			writeOAuthHTML(w, http.StatusBadRequest, "Sign-in link expired", "Return to the browser tab opened by this sign-in attempt.", false)
			return
		}
		if q.Get("code") == "" && q.Get("error") == "" {
			writeOAuthHTML(w, http.StatusBadRequest, "Login failed", "Missing authorization code. Start sign-in again in Solomon.", false)
			return
		}
		mu.Lock()
		if consumed {
			mu.Unlock()
			writeOAuthHTML(w, http.StatusConflict, "Sign-in already received", "Return to Solomon to see the result.", false)
			return
		}
		consumed = true
		mu.Unlock()
		var result loginResult
		if q.Get("error") != "" {
			// Provider error codes are safe to show; do not echo arbitrary browser input.
			result.err = errors.New("Claude sign-in was denied or canceled; run /connect to try again")
		} else {
			result.tokens, result.err = exchangeCode(ctx, q.Get("code"), pkce.State, pkce.Verifier, redirect)
		}
		if result.err != nil {
			writeOAuthHTML(w, http.StatusBadRequest, "Login failed", "Authorization could not be completed. Return to Solomon and start sign-in again.", false)
		} else {
			writeOAuthHTML(w, http.StatusOK, "Login successful", "You can close this tab and return to Solomon.", true)
		}
		_ = http.NewResponseController(w).Flush()
		select {
		case results <- result:
		case <-ctx.Done():
		}
	})
}

func writeOAuthHTML(w http.ResponseWriter, status int, title, body string, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	fmt.Fprint(w, logo.OAuthCallbackHTML(title, body, ok))
}

func openBrowser(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("open browser: empty URL")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
