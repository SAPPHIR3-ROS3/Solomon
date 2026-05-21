package llm

import (
	"bytes"
	"context"
	"net/http"
)

func anthropicHTTPDefault() *http.Client {
	return &http.Client{}
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
