package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type searxEngine struct{}

func applySearxHTTPHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*;q=0.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

func searxEnumerateSearchURLs(instBase string, queryText string) []string {
	b := strings.TrimSuffix(strings.TrimSpace(instBase), "/")
	if b == "" {
		return nil
	}
	pu, err := url.Parse(b)
	if err != nil || pu.Scheme == "" || pu.Host == "" {
		return nil
	}
	pathTail := strings.Trim(strings.TrimPrefix(pu.Path, "/"), "/")
	suffixes := []string{"/search"}
	if pathTail == "" {
		suffixes = append(suffixes, "/searxng/search")
	}
	out := make([]string, 0, len(suffixes))
	for _, suf := range suffixes {
		u2, err := url.Parse(b + suf)
		if err != nil {
			continue
		}
		q := u2.Query()
		q.Set("q", queryText)
		q.Set("format", "json")
		u2.RawQuery = q.Encode()
		out = append(out, u2.String())
	}
	return out
}

func searxPerform(ctx context.Context, baseURL string, req Request) (Response, error) {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return Response{}, fmt.Errorf("searxng: empty base URL")
	}
	urls := searxEnumerateSearchURLs(baseURL, req.Query)
	if len(urls) == 0 {
		return Response{}, fmt.Errorf("searxng: invalid base URL")
	}
	var lastErr error
	for _, full := range urls {
		out, err := searxPerformGET(ctx, full, baseURL, req)
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("searxng: no reachable search URL")
	}
	return Response{}, lastErr
}

func searxPerformGET(ctx context.Context, fullURL string, metaBase string, req Request) (Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return Response{}, fmt.Errorf("searxng: build request: %w", err)
	}
	applySearxHTTPHeaders(httpReq)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("searxng: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Response{}, fmt.Errorf("searxng: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Response{}, fmt.Errorf("searxng: HTTP %d", resp.StatusCode)
	}

	var decoded struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Response{}, fmt.Errorf("searxng: json: %w", err)
	}
	max := req.MaxResults
	if max <= 0 {
		max = 10
	}
	out := Response{Hits: make([]Hit, 0), SearxBaseURL: metaBase}
	for _, r := range decoded.Results {
		link := strings.TrimSpace(r.URL)
		if link == "" {
			continue
		}
		if len(out.Hits) >= max {
			out.HasMore = true
			break
		}
		out.Hits = append(out.Hits, Hit{
			Title:   strings.TrimSpace(r.Title),
			URL:     link,
			Snippet: strings.TrimSpace(r.Content),
		})
	}
	return out, nil
}

func (searxEngine) Search(ctx context.Context, req Request) (Response, error) {
	explicit := ""
	if req.Extras != nil {
		if raw, ok := req.Extras["baseURL"]; ok {
			if s, ok := raw.(string); ok {
				explicit = strings.TrimSpace(s)
			}
		}
	}
	if explicit == "" {
		return Response{}, fmt.Errorf("searxng: set web_search_base_url in config or extras.baseURL")
	}
	return searxPerform(ctx, strings.TrimSuffix(explicit, "/"), req)
}

func init() {
	Register("searxng", searxEngine{})
}
