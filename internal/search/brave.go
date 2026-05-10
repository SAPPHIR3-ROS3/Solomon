package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type braveEngine struct{}

func (braveEngine) Search(ctx context.Context, req Request) (Response, error) {
	apiKey, err := extrasString(req.Extras, "apiKey")
	if err != nil {
		return Response{}, fmt.Errorf("brave: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return Response{}, fmt.Errorf("brave: extras.apiKey is required (Brave Search subscription token)")
	}

	n := req.MaxResults
	if n <= 0 {
		n = 10
	}
	if n > 50 {
		n = 50
	}

	u, err := url.Parse("https://api.search.brave.com/res/v1/web/search")
	if err != nil {
		return Response{}, fmt.Errorf("brave: parse url: %w", err)
	}
	q := u.Query()
	q.Set("q", req.Query)
	q.Set("count", strconv.Itoa(n))
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Response{}, fmt.Errorf("brave: build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-Subscription-Token", apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("brave: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Response{}, fmt.Errorf("brave: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Response{}, fmt.Errorf("brave: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Response{}, fmt.Errorf("brave: json: %w", err)
	}
	out := Response{Hits: make([]Hit, 0, len(decoded.Web.Results))}
	for _, r := range decoded.Web.Results {
		link := strings.TrimSpace(r.URL)
		if link == "" {
			continue
		}
		out.Hits = append(out.Hits, Hit{
			Title:   strings.TrimSpace(r.Title),
			URL:     link,
			Snippet: strings.TrimSpace(r.Description),
		})
		if len(out.Hits) >= n {
			break
		}
	}
	out.HasMore = len(decoded.Web.Results) > len(out.Hits)
	return out, nil
}

func init() {
	Register("brave", braveEngine{})
}
