package search

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
)

const (
	CloakAdapterName         = "cloak"
	defaultCloakSearchURL    = "https://www.google.com/search?q=%s"
	nativeCloakSnapshotLimit = 1_000_000
)

// NativeCloakAdapter is Solomon's local CloakBrowser fallback. The browser
// process is owned by internal/cloak; this adapter only turns a rendered
// search page into the provider-neutral search response.
type NativeCloakAdapter struct {
	browser cloak.Browser
}

func NewNativeCloakAdapter(browser cloak.Browser) (*NativeCloakAdapter, error) {
	if browser == nil {
		return nil, fmt.Errorf("native cloak adapter: browser is required")
	}
	return &NativeCloakAdapter{browser: browser}, nil
}

func (a *NativeCloakAdapter) Search(ctx context.Context, req Request) (Response, error) {
	if a == nil || a.browser == nil {
		return Response{}, fmt.Errorf("native cloak: browser unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return Response{}, fmt.Errorf("native cloak: empty query")
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = defaultSearchResults
	}
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "web search fallback: " + query
	}
	searchURL, err := cloakSearchURL(query, maxResults, req.Extras)
	if err != nil {
		return Response{}, fmt.Errorf("native cloak: %w", err)
	}

	tabID, err := a.browser.NewTab(ctx, intent)
	if err != nil {
		return Response{}, fmt.Errorf("native cloak new tab: %w", err)
	}
	defer closeNativeCloakTab(a.browser, tabID, intent)

	navigation, err := a.browser.Navigate(ctx, tabID, searchURL, intent)
	if err != nil {
		return Response{}, fmt.Errorf("native cloak navigate: %w", err)
	}
	if navigation.Status >= 400 {
		return Response{}, fmt.Errorf("native cloak search page returned HTTP %d", navigation.Status)
	}
	snapshot, err := a.browser.Snapshot(ctx, tabID, intent, cloak.SnapshotOptions{
		MaxCharacters: nativeCloakSnapshotLimit,
		IncludeHTML:   true,
	})
	if err != nil {
		return Response{}, fmt.Errorf("native cloak snapshot: %w", err)
	}

	hits := nativeCloakHits(snapshot, searchURL, maxResults)
	if len(hits) == 0 {
		return Response{}, fmt.Errorf("native cloak: %w", ErrNoResults)
	}
	hasMore := maxResults > 0 && len(hits) > maxResults
	if hasMore {
		hits = hits[:maxResults]
	}
	response := Response{
		Engine:  CloakAdapterName,
		Hits:    hits,
		HasMore: hasMore,
		Metadata: &ResponseMetadata{
			Provider: CloakAdapterName,
			Adapter:  CloakAdapterName,
			ProviderData: map[string]any{
				"requestedURL": searchURL,
				"finalURL":     firstNonEmpty(snapshot.URL, navigation.URL),
				"title":        firstNonEmpty(snapshot.Title, navigation.Title),
				"status":       firstStatus(snapshot.Status, navigation.Status),
				"contentType":  firstNonEmpty(snapshot.ContentType, navigation.ContentType),
				"linkCount":    len(snapshot.Links),
				"truncated":    snapshot.Truncated,
			},
		},
	}
	return response, nil
}

func cloakSearchURL(query string, maxResults int, extras map[string]any) (string, error) {
	template := extraString(extras, "searchURL", "search_url")
	if template == "" {
		template = defaultCloakSearchURL
	}
	escaped := url.QueryEscape(query)
	if strings.Contains(template, "%s") {
		rendered := strings.Replace(template, "%s", escaped, 1)
		if maxResults <= 0 {
			return rendered, nil
		}
		if parsed, parseErr := url.Parse(rendered); parseErr == nil && parsed.Host != "" {
			values := parsed.Query()
			if values.Get("num") == "" {
				values.Set("num", fmt.Sprintf("%d", maxResults))
				parsed.RawQuery = values.Encode()
			}
			return parsed.String(), nil
		}
		return rendered, nil
	}
	parsed, err := url.Parse(template)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid search URL %q", template)
	}
	values := parsed.Query()
	values.Set("q", query)
	if maxResults > 0 {
		values.Set("num", fmt.Sprintf("%d", maxResults))
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func nativeCloakHits(snapshot cloak.Snapshot, searchPage string, maxResults int) []Hit {
	searchURL, _ := url.Parse(strings.TrimSpace(searchPage))
	hits := make([]Hit, 0, len(snapshot.Links))
	for _, link := range snapshot.Links {
		resultURL, ok := normalizeCloakResultURL(link.URL, searchURL)
		if !ok {
			continue
		}
		appendHit(&hits, Hit{
			Title:   link.Title,
			URL:     resultURL,
			Snippet: link.Snippet,
			Metadata: map[string]any{
				"source": "cloakbrowser",
			},
		})
	}
	if len(hits) == 0 {
		for _, hit := range parseTextHits(snapshot.Text) {
			resultURL, ok := normalizeCloakResultURL(hit.URL, searchURL)
			if !ok {
				continue
			}
			hit.URL = resultURL
			appendHit(&hits, hit)
		}
	}
	return hits
}

func normalizeCloakResultURL(raw string, searchPage *url.URL) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if isKnownSearchHost(parsed.Hostname()) {
		for _, key := range []string{"q", "url", "u", "target"} {
			if target := strings.TrimSpace(parsed.Query().Get(key)); target != "" {
				if normalized, ok := normalizeCloakResultURL(target, searchPage); ok {
					return normalized, true
				}
			}
		}
		return "", false
	}
	if searchPage != nil && isKnownSearchHost(searchPage.Hostname()) && strings.EqualFold(parsed.Hostname(), searchPage.Hostname()) {
		return "", false
	}
	return normalizeHitURL(parsed.String())
}

func isKnownSearchHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(host, "www.")
	return host == "google.com" || strings.HasSuffix(host, ".google.com") ||
		host == "bing.com" || strings.HasSuffix(host, ".bing.com") ||
		host == "duckduckgo.com" || strings.HasSuffix(host, ".duckduckgo.com") ||
		host == "search.brave.com" || host == "search.yahoo.com" ||
		host == "yahoo.com" || strings.HasSuffix(host, ".yahoo.com") ||
		host == "yandex.com" || strings.HasSuffix(host, ".yandex.com")
}

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

var _ Engine = (*NativeCloakAdapter)(nil)
