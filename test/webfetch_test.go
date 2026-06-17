package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
)

func TestWebFetchDomainBlocked(t *testing.T) {
	t.Parallel()
	blocked, rule := webfetch.DomainBlockedForTest("www.linkedin.com", []string{"linkedin.com"})
	if !blocked || rule != "linkedin.com" {
		t.Fatalf("blocked=%v rule=%q", blocked, rule)
	}
	blocked, _ = webfetch.DomainBlockedForTest("example.com", []string{"linkedin.com"})
	if blocked {
		t.Fatal("expected allowed host")
	}
}

func TestWebFetchClassifyHTTPStatus(t *testing.T) {
	t.Parallel()
	got := webfetch.ClassifyHTTPStatusForTest(429, nil)
	if got != "rate limited" {
		t.Fatalf("429: %q", got)
	}
	got = webfetch.ClassifyHTTPStatusForTest(403, []byte(`<html>cf-ray: abc Attention Required`))
	if got != "cloudflare challenge" {
		t.Fatalf("cf: %q", got)
	}
}

func TestWebFetchRetryOn503(t *testing.T) {
	webfetch.ResetSharedClientForTest()
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 2 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>OK</title></head><body>hello</body></html>"))
	}))
	defer srv.Close()

	res, err := webfetch.FetchURL(context.Background(), srv.URL, 10, nil)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if hits < 2 {
		t.Fatalf("expected retry, hits=%d", hits)
	}
	if !strings.Contains(res.Markdown, "hello") {
		t.Fatalf("markdown: %q", res.Markdown)
	}
}

func TestWebFetchBlockedDomainConfig(t *testing.T) {
	webfetch.ResetSharedClientForTest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Root{
		WebFetch: config.WebFetchConfig{
			BlockedDomains: []string{"127.0.0.1"},
		},
	}
	_, err := webfetch.FetchURL(context.Background(), srv.URL, 5, cfg)
	if err == nil || !strings.Contains(err.Error(), "domain blocked") {
		t.Fatalf("expected blocked error, got %v", err)
	}
}

func TestWebFetchUserAgentHeader(t *testing.T) {
	webfetch.ResetSharedClientForTest()
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, err := webfetch.FetchURL(context.Background(), srv.URL, 5, nil)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(ua, "Chrome/131") || !strings.Contains(ua, "github.com/SAPPHIR3-ROS3/Solomon") {
		t.Fatalf("user-agent: %q", ua)
	}
}
