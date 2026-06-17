package webfetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

const (
	MaxBodyBytes    = 5 * 1024 * 1024
	DefaultTimeoutS = 30
	MaxTimeoutSecs  = 120
	UserAgent       = defaultUserAgent
	maxFetchRetries = 3
)

type Result struct {
	URL         string
	Status      int
	ContentType string
	Markdown    string
	Title       string
}

func FetchURL(ctx context.Context, rawURL string, timeoutSec int, cfg *config.Root) (Result, error) {
	configureRedirects(config.EffectiveWebFetchMaxRedirects(cfg))
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return Result{}, fmt.Errorf("empty url")
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return Result{}, fmt.Errorf("invalid url")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return Result{}, fmt.Errorf("only http and https URLs are allowed")
	}
	if blocked, rule := domainBlocked(u.Hostname(), config.EffectiveWebFetchBlockedDomains(cfg)); blocked {
		return Result{}, fmt.Errorf("domain blocked (%s): %s", rule, u.Host)
	}
	sec := DefaultTimeoutS
	if timeoutSec > 0 {
		sec = timeoutSec
		if sec > MaxTimeoutSecs {
			sec = MaxTimeoutSecs
		}
	}
	var lastErr error
	for attempt := 1; attempt <= maxFetchRetries; attempt++ {
		if attempt > 1 {
			wait := retryWait(attempt, lastErr)
			if wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return Result{}, ctx.Err()
				case <-timer.C:
				}
			}
		}
		res, err := fetchOnce(ctx, rawURL, sec, cfg)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if attempt >= maxFetchRetries || !retryableFetchError(err) {
			return Result{}, err
		}
	}
	if lastErr != nil {
		return Result{}, lastErr
	}
	return Result{}, fmt.Errorf("fetch failed")
}

func fetchOnce(ctx context.Context, rawURL string, timeoutSec int, cfg *config.Root) (Result, error) {
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}
	setFetchHeaders(req, cfg)
	resp, err := sharedHTTPClient().Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return Result{}, err
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return Result{}, fmt.Errorf("request timeout: %w", err)
		}
		return Result{}, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("read body: %w", err)
	}
	if len(body) > MaxBodyBytes {
		return Result{}, fmt.Errorf("response body exceeds %d bytes", MaxBodyBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Result{}, statusError(resp, body)
	}
	rawStr := string(body)
	ct := mimeFromContentType(resp.Header.Get("Content-Type"))
	md, err := bytesToMarkdown(rawStr, ct)
	if err != nil {
		return Result{}, err
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	title := extractHTMLTitle(rawStr)
	return Result{
		URL:         finalURL,
		Status:      resp.StatusCode,
		ContentType: strings.TrimSpace(resp.Header.Get("Content-Type")),
		Markdown:    md,
		Title:       title,
	}, nil
}

func setFetchHeaders(req *http.Request, cfg *config.Root) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/markdown,text/plain;q=0.9,application/json;q=0.8,*/*;q=0.1")
	req.Header.Set("Accept-Language", config.EffectiveWebFetchAcceptLanguage(cfg))
	req.Header.Set("User-Agent", effectiveUserAgent(cfg))
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
}

func statusError(resp *http.Response, body []byte) error {
	msg := classifyHTTPStatus(resp.StatusCode, body)
	err := fmt.Errorf("HTTP %d (%s)", resp.StatusCode, msg)
	return &fetchHTTPError{
		status:     resp.StatusCode,
		retryAfter: transport.ParseRetryAfterHeader(resp),
		err:        err,
	}
}

type fetchHTTPError struct {
	status     int
	retryAfter time.Duration
	err        error
}

func (e *fetchHTTPError) Error() string {
	if e == nil || e.err == nil {
		return "http error"
	}
	return e.err.Error()
}

func (e *fetchHTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func classifyHTTPStatus(status int, body []byte) string {
	snippet := strings.ToLower(string(body))
	if len(snippet) > 4096 {
		snippet = snippet[:4096]
	}
	switch status {
	case 403:
		if looksCloudflare(snippet) {
			return "cloudflare challenge"
		}
		return "forbidden"
	case 429:
		return "rate limited"
	case 408:
		return "request timeout"
	case 502:
		return "bad gateway"
	case 503:
		if looksCloudflare(snippet) {
			return "cloudflare unavailable"
		}
		return "service unavailable"
	default:
		if looksCloudflare(snippet) {
			return "cloudflare challenge"
		}
		return "error response"
	}
}

func looksCloudflare(snippet string) bool {
	return strings.Contains(snippet, "cf-ray") ||
		strings.Contains(snippet, "cloudflare") && strings.Contains(snippet, "attention required")
}

func retryableFetchError(err error) bool {
	var he *fetchHTTPError
	if errors.As(err, &he) {
		switch he.status {
		case 408, 429, 502, 503:
			return true
		default:
			return false
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func retryWait(attempt int, err error) time.Duration {
	var he *fetchHTTPError
	if errors.As(err, &he) && he.retryAfter > 0 {
		return he.retryAfter
	}
	base := time.Second
	wait := base << (attempt - 2)
	if wait < base {
		wait = base
	}
	if wait > 30*time.Second {
		wait = 30 * time.Second
	}
	return wait
}

func domainBlocked(host string, rules []string) (bool, string) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || len(rules) == 0 {
		return false, ""
	}
	for _, rule := range rules {
		rule = strings.TrimSpace(strings.ToLower(rule))
		if rule == "" {
			continue
		}
		if host == rule {
			return true, rule
		}
		if strings.HasPrefix(rule, ".") {
			if strings.HasSuffix(host, rule) {
				return true, rule
			}
			continue
		}
		if strings.HasSuffix(host, "."+rule) {
			return true, rule
		}
	}
	return false, ""
}

func mimeFromContentType(h string) string {
	h = strings.TrimSpace(strings.Split(h, ";")[0])
	return strings.ToLower(h)
}

func bytesToMarkdown(body string, mime string) (string, error) {
	switch {
	case strings.Contains(mime, "html"):
		out, err := htmltomarkdown.ConvertString(body)
		if err != nil {
			return "", fmt.Errorf("html to markdown: %w", err)
		}
		return strings.TrimSpace(out), nil
	case mime == "" && looksLikeHTMLSnippet(body):
		out, err := htmltomarkdown.ConvertString(body)
		if err != nil {
			return "", fmt.Errorf("html to markdown: %w", err)
		}
		return strings.TrimSpace(out), nil
	case mime == "":
		return fenceMarkdown("", strings.TrimRight(body, "\r\n")), nil
	case strings.Contains(mime, "markdown"):
		return strings.TrimSpace(body), nil
	case strings.Contains(mime, "json"):
		return fenceMarkdown("json", body), nil
	case strings.Contains(mime, "xml") || mime == "text/xml" || mime == "application/xml":
		return fenceMarkdown("xml", body), nil
	case strings.HasPrefix(mime, "text/"):
		return fenceMarkdown("", strings.TrimRight(body, "\r\n")), nil
	default:
		if looksLikeHTMLSnippet(body) {
			out, err := htmltomarkdown.ConvertString(body)
			if err == nil && strings.TrimSpace(out) != "" {
				return strings.TrimSpace(out), nil
			}
		}
		return "", fmt.Errorf("unsupported content-type %q (not text/html-like)", mime)
	}
}

func looksLikeHTMLSnippet(s string) bool {
	t := strings.TrimLeft(s, " \n\r\t")
	if len(t) == 0 {
		return false
	}
	head := strings.ToLower(t)
	if len(head) > 1024 {
		head = head[:1024]
	}
	return strings.HasPrefix(head, "<!doctype") || strings.Contains(head, "<html") ||
		strings.Contains(head, "<head") || strings.Contains(head, "<body")
}

func fenceMarkdown(lang, body string) string {
	body = strings.TrimRight(body, "\r\n") + "\n"
	if lang == "" {
		return "```\n" + body + "```"
	}
	return "```" + lang + "\n" + body + "```"
}

func extractHTMLTitle(html string) string {
	lower := strings.ToLower(html)
	const open = "<title>"
	const close = "</title>"
	i := strings.Index(lower, open)
	if i < 0 {
		return ""
	}
	i += len(open)
	j := strings.Index(lower[i:], close)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(html[i : i+j])
}
