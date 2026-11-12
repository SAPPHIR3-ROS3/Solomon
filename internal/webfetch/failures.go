package webfetch

import (
	"context"
	"errors"
	"net"
	"strings"
)

func classifyFetchFailure(err error) (string, bool) {
	if errors.Is(err, context.Canceled) {
		return "cancelled", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", true
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) && dnsError.IsNotFound {
		return "dns", false
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		if networkError.Timeout() {
			return "timeout", true
		}
		return "network", true
	}
	var httpError *fetchHTTPError
	if errors.As(err, &httpError) {
		switch httpError.status {
		case 408, 429, 500, 502, 503, 504:
			return "http", true
		default:
			return "http", false
		}
	}
	if errors.Is(err, ErrNoContent) {
		return "content", false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{"goaway", "connection reset", "unexpected eof", "connection refused"} {
		if strings.Contains(text, marker) {
			return "network", true
		}
	}
	return "provider", false
}
