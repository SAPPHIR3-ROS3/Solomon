package llm

import (
	"bytes"
	"context"
	"net/http"
	"strings"
)

func anthropicHTTPDefault() *http.Client {
	return &http.Client{Timeout: 0}
}

func anthropicHTTPError(resp *http.Response, body []byte) error {
	if resp == nil {
		return NewProviderHTTPError(0, string(body), 0)
	}
	return NewProviderHTTPError(resp.StatusCode, strings.TrimSpace(string(body)), parseRetryAfterHeader(resp))
}

func anthropicHTTPNew(ctx context.Context, url string, body []byte, auth AnthropicAuth) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	auth.ApplyTo(req)
	return req, nil
}
