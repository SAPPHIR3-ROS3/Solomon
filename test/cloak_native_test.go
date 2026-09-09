package test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
)

type cloakBrowserTestDouble struct {
	navigation cloak.Navigation
	snapshot   cloak.Snapshot
	calls      []string
	lastURL    string
	lastOpts   cloak.SnapshotOptions
}

func (b *cloakBrowserTestDouble) NewTab(_ context.Context, _ string) (string, error) {
	b.calls = append(b.calls, "newTab")
	return "t1", nil
}

func (b *cloakBrowserTestDouble) Navigate(_ context.Context, tabID, rawURL, _ string) (cloak.Navigation, error) {
	b.calls = append(b.calls, "navigate:"+tabID)
	b.lastURL = rawURL
	return b.navigation, nil
}

func (b *cloakBrowserTestDouble) Snapshot(_ context.Context, tabID, _ string, options cloak.SnapshotOptions) (cloak.Snapshot, error) {
	b.calls = append(b.calls, "snapshot:"+tabID)
	b.lastOpts = options
	return b.snapshot, nil
}

func (b *cloakBrowserTestDouble) CloseTab(_ context.Context, tabID, _ string) error {
	b.calls = append(b.calls, "closeTab:"+tabID)
	return nil
}

func (b *cloakBrowserTestDouble) Close() error { return nil }

func TestCloakSnapshotParserRunsInGo(t *testing.T) {
	parsed := cloak.ParseSnapshotHTML(`<!doctype html>
<html><head><title> Example page </title></head><body>
  <article><a href="/docs/one"><span>First result</span></a><p>First excerpt.</p></article>
  <div><a aria-label="Second result" href="https://example.com/two"></a></div>
  <a href="javascript:void(0)">ignored</a>
  <a href="/docs/one">duplicate</a>
</body></html>`, "https://example.com/search?q=solomon")

	if parsed.Title != "Example page" {
		t.Fatalf("title=%q", parsed.Title)
	}
	if !strings.Contains(parsed.Text, "First result") || !strings.Contains(parsed.Text, "First excerpt.") {
		t.Fatalf("text=%q", parsed.Text)
	}
	if len(parsed.Links) != 2 {
		t.Fatalf("links=%+v; want two links", parsed.Links)
	}
	if parsed.Links[0].Title != "First result" || parsed.Links[0].URL != "https://example.com/docs/one" || parsed.Links[0].Snippet == "" {
		t.Fatalf("first link=%+v", parsed.Links[0])
	}
	if parsed.Links[1].Title != "Second result" || parsed.Links[1].URL != "https://example.com/two" {
		t.Fatalf("second link=%+v", parsed.Links[1])
	}
}

func TestCloakNodeBridgeOnlyKeepsBrowserBoundary(t *testing.T) {
	root := repositoryRootForTestLayout(t)
	bridge, err := os.ReadFile(filepath.Join(root, "internal", "cloak", "bridge.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(bridge)
	if !strings.Contains(source, `import("cloakbrowser")`) {
		t.Fatal("bridge does not use the official cloakbrowser package")
	}
	for _, forbidden := range []string{"querySelectorAll", "document.body", "innerText"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("DOM extraction %q leaked into the Node sidecar", forbidden)
		}
	}
}

func TestCloakClientRequiresOfficialNPMInstall(t *testing.T) {
	_, err := cloak.NewClient(cloak.Options{InstallDir: t.TempDir()})
	if !errors.Is(err, cloak.ErrUnavailable) {
		t.Fatalf("error=%v; want cloak.ErrUnavailable", err)
	}
}

func TestNativeCloakSearchAdapterUsesGoShapedResults(t *testing.T) {
	browser := &cloakBrowserTestDouble{
		navigation: cloak.Navigation{URL: "https://www.google.com/search?q=Solomon", Status: 200, ContentType: "text/html"},
		snapshot: cloak.Snapshot{
			URL:    "https://www.google.com/search?q=Solomon",
			Title:  "Google",
			Status: 200,
			Links: []cloak.Link{
				{Title: "Solomon docs", URL: "https://www.google.com/url?q=https%3A%2F%2Fexample.com%2Fsolomon", Snippet: "Docs excerpt"},
				{Title: "Second", URL: "https://example.com/second"},
				{Title: "Duplicate", URL: "https://example.com/second"},
				{Title: "Third", URL: "https://example.com/third"},
			},
		},
	}
	adapter, err := search.NewNativeCloakAdapter(browser)
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.Search(context.Background(), search.Request{
		Query:      "Solomon",
		MaxResults: 2,
		Intent:     "find Solomon documentation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Hits) != 2 || !response.HasMore {
		t.Fatalf("response=%+v", response)
	}
	if response.Hits[0].URL != "https://example.com/solomon" || response.Hits[0].Snippet != "Docs excerpt" {
		t.Fatalf("hits=%+v", response.Hits)
	}
	if response.Metadata == nil || response.Metadata.Adapter != search.CloakAdapterName || response.Metadata.ProviderData["status"] != 200 {
		t.Fatalf("metadata=%+v", response.Metadata)
	}
	parsedSearchURL, err := url.Parse(browser.lastURL)
	if err != nil || parsedSearchURL.Query().Get("q") != "Solomon" || parsedSearchURL.Query().Get("num") != "2" {
		t.Fatalf("search URL=%q", browser.lastURL)
	}
	if !browser.lastOpts.IncludeHTML {
		t.Fatalf("snapshot options=%+v", browser.lastOpts)
	}
	if len(browser.calls) != 4 || browser.calls[0] != "newTab" || browser.calls[3] != "closeTab:t1" {
		t.Fatalf("browser calls=%v", browser.calls)
	}
}

func TestNativeCloakSearchAdapterRejectsEmptyRenderedResults(t *testing.T) {
	browser := &cloakBrowserTestDouble{snapshot: cloak.Snapshot{Text: "No results"}}
	adapter, err := search.NewNativeCloakAdapter(browser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Search(context.Background(), search.Request{Query: "nothing"})
	if !errors.Is(err, search.ErrNoResults) {
		t.Fatalf("error=%v; want ErrNoResults", err)
	}
	if len(browser.calls) != 4 {
		t.Fatalf("browser calls=%v; want tab cleanup", browser.calls)
	}
}

func TestNativeCloakFetcherConvertsRenderedHTMLInGo(t *testing.T) {
	browser := &cloakBrowserTestDouble{
		navigation: cloak.Navigation{URL: "https://example.com/final", Status: 200, ContentType: "text/html; charset=utf-8", Title: "Example"},
		snapshot: cloak.Snapshot{
			URL:         "https://example.com/final",
			Title:       "Example",
			Status:      200,
			ContentType: "text/html; charset=utf-8",
			HTML:        `<!doctype html><html><head><title>Example</title></head><body><h1>Hello</h1><p>Rendered content.</p></body></html>`,
		},
	}
	fetcher, err := webfetch.NewNativeCloakFetcher(browser)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fetcher.Fetch(context.Background(), webfetch.Request{URL: "https://example.com/page", Intent: "read protected page"})
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://example.com/final" || result.Status != 200 || result.Title != "Example" {
		t.Fatalf("result=%+v", result)
	}
	if !strings.Contains(result.Markdown, "Hello") || !strings.Contains(result.Markdown, "Rendered content.") {
		t.Fatalf("markdown=%q", result.Markdown)
	}
	if result.Metadata == nil || result.Metadata.Adapter != webfetch.CloakAdapterName || result.Metadata.ProviderData["finalURL"] != result.URL {
		t.Fatalf("metadata=%+v", result.Metadata)
	}
	if !browser.lastOpts.IncludeHTML || browser.lastOpts.MaxCharacters != webfetch.MaxBodyBytes {
		t.Fatalf("snapshot options=%+v", browser.lastOpts)
	}
	if len(browser.calls) != 4 {
		t.Fatalf("browser calls=%v; want tab cleanup", browser.calls)
	}
}

func TestNativeCloakFetcherRejectsEmptyRenderedContent(t *testing.T) {
	browser := &cloakBrowserTestDouble{snapshot: cloak.Snapshot{Status: 200}}
	fetcher, err := webfetch.NewNativeCloakFetcher(browser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fetcher.Fetch(context.Background(), webfetch.Request{URL: "https://example.com/empty"})
	if !errors.Is(err, webfetch.ErrNoContent) {
		t.Fatalf("error=%v; want ErrNoContent", err)
	}
	if len(browser.calls) != 4 {
		t.Fatalf("browser calls=%v; want tab cleanup", browser.calls)
	}
}
