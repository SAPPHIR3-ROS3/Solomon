package llm

import (
	"bytes"
	"context"
	"net/http"
)

func anthropicHTTPDefault() *http.Client {
	return &http.Client{}
}

func anthropicHTTPNew(ctx context.Context, url string, body []byte, apiKey string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	return req, nil
}
