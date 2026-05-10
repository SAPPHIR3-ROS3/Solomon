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

const bingDefaultEndpoint = "https://api.bing.microsoft.com/v7.0/search"

type bingEngine struct{}

func (bingEngine) Search(ctx context.Context, req Request) (Response, error) {
	apiKey, err := extrasString(req.Extras, "apiKey")
	if err != nil {
		return Response{}, fmt.Errorf("bing: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return Response{}, fmt.Errorf("bing: extras.apiKey is required (Bing/Azure subscription key)")
	}

	ep := strings.TrimSpace(extrasOptionalString(req.Extras, "endpoint"))
	if ep == "" {
		ep = bingDefaultEndpoint
	}
	uu, err := url.Parse(ep)
	if err != nil || uu.Scheme == "" || uu.Host == "" {
		return Response{}, fmt.Errorf("bing: invalid extras.endpoint")
	}

	n := req.MaxResults
	if n <= 0 {
		n = 10
	}
	if n > 50 {
		n = 50
	}
	q := uu.Query()
	q.Set("q", req.Query)
	q.Set("count", strconv.Itoa(n))
	uu.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, uu.String(), nil)
	if err != nil {
		return Response{}, fmt.Errorf("bing: build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("bing: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Response{}, fmt.Errorf("bing: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Response{}, fmt.Errorf("bing: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Response{}, fmt.Errorf("bing: json: %w", err)
	}
	out := Response{Hits: make([]Hit, 0, len(decoded.WebPages.Value))}
	for _, r := range decoded.WebPages.Value {
		link := strings.TrimSpace(r.URL)
		if link == "" {
			continue
		}
		out.Hits = append(out.Hits, Hit{
			Title:   strings.TrimSpace(r.Name),
			URL:     link,
			Snippet: strings.TrimSpace(r.Snippet),
		})
		if len(out.Hits) >= n {
			break
		}
	}
	out.HasMore = len(decoded.WebPages.Value) >= n
	return out, nil
}

func init() {
	Register("bing", bingEngine{})
}
