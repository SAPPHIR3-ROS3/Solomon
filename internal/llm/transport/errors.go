package transport

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ProviderHTTPError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *ProviderHTTPError) Error() string {
	if e.Message != "" {
		return "API HTTP " + strconv.Itoa(e.StatusCode) + ": " + e.Message
	}
	return "API HTTP " + strconv.Itoa(e.StatusCode)
}

func NewProviderHTTPError(status int, message string, retryAfter time.Duration) *ProviderHTTPError {
	return &ProviderHTTPError{StatusCode: status, Message: strings.TrimSpace(message), RetryAfter: retryAfter}
}

func ParseRetryAfterHeader(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	v := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}
