package anthropic

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

func httpDefault() *http.Client {
	return &http.Client{Timeout: 0}
}

func httpError(resp *http.Response, body []byte) error {
	if resp == nil {
		return transport.NewProviderHTTPError(0, string(body), 0)
	}
	return transport.NewProviderHTTPError(resp.StatusCode, strings.TrimSpace(string(body)), transport.ParseRetryAfterHeader(resp))
}

func httpNew(ctx context.Context, url string, body []byte, auth Auth, stream bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		auth.ApplyStreamTo(req)
	} else {
		auth.ApplyTo(req)
	}
	return req, nil
}
