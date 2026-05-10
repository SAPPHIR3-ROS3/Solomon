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

type googlePSEEngine struct{}

func (googlePSEEngine) Search(ctx context.Context, req Request) (Response, error) {
	apiKey, err := extrasString(req.Extras, "apiKey")
	if err != nil {
		return Response{}, fmt.Errorf("googlepse: %w", err)
	}
	cxVal, err := extrasString(req.Extras, "cx")
	if err != nil {
		return Response{}, fmt.Errorf("googlepse: cx: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)
	cxVal = strings.TrimSpace(cxVal)
	if apiKey == "" || cxVal == "" {
		return Response{}, fmt.Errorf("googlepse: extras.apiKey and extras.cx are required non-empty strings")
	}

	num := req.MaxResults
	if num <= 0 {
		num = 10
	}
	if num > 10 {
		num = 10
	}

	u, err := url.Parse("https://www.googleapis.com/customsearch/v1")
	if err != nil {
		return Response{}, fmt.Errorf("googlepse: parse url: %w", err)
	}
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("cx", cxVal)
	q.Set("q", req.Query)
	q.Set("num", strconv.Itoa(num))
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Response{}, fmt.Errorf("googlepse: build request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Solomon-webSearch/1.0")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("googlepse: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return Response{}, fmt.Errorf("googlepse: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Response{}, fmt.Errorf("googlepse: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Response{}, fmt.Errorf("googlepse: json: %w", err)
	}
	out := Response{Hits: make([]Hit, 0, len(decoded.Items))}
	for _, it := range decoded.Items {
		link := strings.TrimSpace(it.Link)
		if link == "" {
			continue
		}
		out.Hits = append(out.Hits, Hit{
			Title:   strings.TrimSpace(it.Title),
			URL:     link,
			Snippet: strings.TrimSpace(it.Snippet),
		})
	}
	return out, nil
}

func init() {
	Register("googlepse", googlePSEEngine{})
}
