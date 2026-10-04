package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// DesktopProxy connects the native WebView to the shared daemon. It adapts the
// native origin to the desktop HTTP origin understood by older running daemons,
// so installing a new GUI does not require interrupting their active sessions.
// The caller must serve this handler exclusively on a loopback listener.
func DesktopProxy() http.Handler {
	return desktopProxy(false)
}

// DesktopFrontend makes the daemon the source of the native client's UI assets.
// Reloading a client after an update loads the daemon's matching frontend.
func DesktopFrontend() http.Handler { return desktopProxy(true) }

func desktopProxy(frontend bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAPI := strings.HasPrefix(r.URL.Path, "/__solomon/") || r.URL.Path == "/health"
		if !isAPI && (!frontend || r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/_solomon")) {
			http.NotFound(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "wails://wails" && origin != "http://wails.localhost" && origin != "https://wails.localhost" {
			http.Error(w, "request origin is not allowed", http.StatusForbidden)
			return
		}
		state, err := LoadState()
		if err != nil {
			http.Error(w, "Solomon daemon is unavailable", http.StatusServiceUnavailable)
			return
		}
		target, err := url.Parse(state.URL)
		if err != nil || target.Host == "" {
			http.Error(w, "invalid Solomon daemon URL", http.StatusServiceUnavailable)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(response *http.Response) error {
			if origin != "" {
				response.Header.Set("Access-Control-Allow-Origin", origin)
				response.Header.Set("Vary", "Origin")
			}
			return nil
		}
		request := r.Clone(r.Context())
		request.Host = target.Host
		if origin != "" {
			request.Header.Set("Origin", "http://wails.localhost")
		}
		proxy.ServeHTTP(w, request)
	})
}
