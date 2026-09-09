package webfetch

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
)

const CloakAdapterName = "cloak"

// NativeCloakFetcher is the local browser fallback. CloakBrowser is used only
// to render the page; content extraction and Markdown conversion remain in Go
// alongside the legacy fetch implementation.
type NativeCloakFetcher struct {
	browser cloak.Browser
}

func NewNativeCloakFetcher(browser cloak.Browser) (*NativeCloakFetcher, error) {
	if browser == nil {
		return nil, fmt.Errorf("native cloak fetcher: browser is required")
	}
	return &NativeCloakFetcher{browser: browser}, nil
}

func (f *NativeCloakFetcher) Fetch(ctx context.Context, req Request) (Result, error) {
	if f == nil || f.browser == nil {
		return Result{}, fmt.Errorf("native cloak: browser unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rawURL := strings.TrimSpace(req.URL)
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Result{}, fmt.Errorf("native cloak: invalid url")
	}
	if scheme := strings.ToLower(parsedURL.Scheme); scheme != "http" && scheme != "https" {
		return Result{}, fmt.Errorf("native cloak: only http and https URLs are allowed")
	}
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "web fetch fallback: " + rawURL
	}

	requestCtx, cancel := context.WithTimeout(ctx, fetchTimeout(req))
	defer cancel()
	tabID, err := f.browser.NewTab(requestCtx, intent)
	if err != nil {
		return Result{}, fmt.Errorf("native cloak new tab: %w", err)
	}
	defer closeNativeCloakTab(f.browser, tabID, intent)

	navigation, err := f.browser.Navigate(requestCtx, tabID, rawURL, intent)
	if err != nil {
		return Result{}, fmt.Errorf("native cloak navigate: %w", err)
	}
	if navigation.Status >= 400 {
		return Result{}, fmt.Errorf("native cloak page returned HTTP %d", navigation.Status)
	}
	snapshot, err := f.browser.Snapshot(requestCtx, tabID, intent, cloak.SnapshotOptions{
		MaxCharacters: MaxBodyBytes,
		IncludeHTML:   true,
	})
	if err != nil {
		return Result{}, fmt.Errorf("native cloak snapshot: %w", err)
	}

	body := strings.TrimSpace(snapshot.HTML)
	contentType := firstNonEmpty(snapshot.ContentType, navigation.ContentType)
	if body == "" {
		body = strings.TrimSpace(snapshot.Text)
		if contentType == "" {
			contentType = "text/plain"
		}
	}
	if body == "" {
		return Result{}, fmt.Errorf("native cloak: %w", ErrNoContent)
	}
	if contentType == "" {
		contentType = "text/html"
	}
	markdown, err := bytesToMarkdown(body, mimeFromContentType(contentType))
	if err != nil {
		return Result{}, fmt.Errorf("native cloak markdown conversion: %w", err)
	}
	if strings.TrimSpace(markdown) == "" {
		return Result{}, fmt.Errorf("native cloak: %w", ErrNoContent)
	}

	finalURL := firstNonEmpty(snapshot.URL, navigation.URL, rawURL)
	title := firstNonEmpty(snapshot.Title, navigation.Title)
	if title == "" && strings.Contains(strings.ToLower(contentType), "html") {
		title = extractHTMLTitle(body)
	}
	return Result{
		URL:         finalURL,
		Status:      firstStatus(snapshot.Status, navigation.Status),
		ContentType: contentType,
		Markdown:    markdown,
		Title:       title,
		Metadata: &Metadata{
			Provider: CloakAdapterName,
			Adapter:  CloakAdapterName,
			ProviderData: map[string]any{
				"requestedURL": rawURL,
				"finalURL":     finalURL,
				"title":        title,
				"status":       firstStatus(snapshot.Status, navigation.Status),
				"contentType":  contentType,
				"truncated":    snapshot.Truncated,
			},
		},
	}, nil
}

var _ Fetcher = (*NativeCloakFetcher)(nil)

func closeNativeCloakTab(browser cloak.Browser, tabID, intent string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = browser.CloseTab(cleanupCtx, tabID, intent)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstStatus(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
