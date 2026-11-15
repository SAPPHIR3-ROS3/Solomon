package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type httpRequestArgs struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	Body           string            `json:"body"`
	BodyBase64     string            `json:"bodyBase64"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	MaxBytes       int64             `json:"maxBytes"`
}

func execHTTPRequest(ctx context.Context, raw json.RawMessage) (any, error) {
	var a httpRequestArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	u, err := url.Parse(a.URL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("httpRequest: a valid http(s) URL is required")
	}
	if a.Method == "" {
		a.Method = http.MethodGet
	}
	if a.TimeoutSeconds == 0 {
		a.TimeoutSeconds = 30
	}
	if a.TimeoutSeconds < 1 || a.TimeoutSeconds > 120 {
		return nil, fmt.Errorf("httpRequest: timeoutSeconds must be between 1 and 120")
	}
	if a.MaxBytes == 0 {
		a.MaxBytes = 1024 * 1024
	}
	if a.MaxBytes < 1 || a.MaxBytes > 2*1024*1024 {
		return nil, fmt.Errorf("httpRequest: maxBytes must be between 1 and 2097152")
	}
	if a.Body != "" && a.BodyBase64 != "" {
		return nil, fmt.Errorf("httpRequest: body and bodyBase64 are mutually exclusive")
	}
	body := []byte(a.Body)
	if a.BodyBase64 != "" {
		body, err = base64.StdEncoding.DecodeString(a.BodyBase64)
		if err != nil {
			return nil, fmt.Errorf("httpRequest: invalid bodyBase64: %w", err)
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(a.TimeoutSeconds)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, a.Method, a.URL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	for key, value := range a.Headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, a.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	truncated := int64(len(data)) > a.MaxBytes
	if truncated {
		data = data[:a.MaxBytes]
	}
	return map[string]any{"url": response.Request.URL.String(), "status": response.StatusCode, "headers": response.Header, "bodyBase64": base64.StdEncoding.EncodeToString(data), "bytes": len(data), "truncated": truncated}, nil
}
