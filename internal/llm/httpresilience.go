package llm

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	openai "github.com/openai/openai-go/v2"
)

var ErrCircuitOpen = errors.New("provider temporarily unavailable")

type ErrorClass int

const (
	ErrorPermanent ErrorClass = iota
	ErrorRetryable
	ErrorCircuitOpen
)

func HostKeyFromBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return strings.TrimSuffix(strings.TrimPrefix(baseURL, "https://"), "/")
	}
	return u.Host
}

func ClassifyAPIError(err error, circuitOpen bool) (ErrorClass, int, time.Duration) {
	if circuitOpen || errors.Is(err, ErrCircuitOpen) {
		return ErrorCircuitOpen, 0, 0
	}
	if err == nil {
		return ErrorPermanent, 0, 0
	}
	if errors.Is(err, ErrStreamAccumulatorRejected) {
		return ErrorPermanent, 0, 0
	}
	var phe *transport.ProviderHTTPError
	if errors.As(err, &phe) {
		if retryableStatus(phe.StatusCode) {
			return ErrorRetryable, phe.StatusCode, phe.RetryAfter
		}
		return ErrorPermanent, phe.StatusCode, 0
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		ra := transport.ParseRetryAfterHeader(apiErr.Response)
		if retryableStatus(apiErr.StatusCode) {
			return ErrorRetryable, apiErr.StatusCode, ra
		}
		return ErrorPermanent, apiErr.StatusCode, 0
	}
	if code := statusCodeFromMessage(err.Error()); code > 0 {
		if retryableStatus(code) {
			return ErrorRetryable, code, 0
		}
		return ErrorPermanent, code, 0
	}
	if isRetryableNetErr(err) {
		return ErrorRetryable, 0, 0
	}
	return ErrorPermanent, 0, 0
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetryableNetErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, sub := range []string{
		"connection reset",
		"connection refused",
		"forcibly closed",
		"broken pipe",
		"unexpected eof",
		"eof",
		"i/o timeout",
		"timeout",
		"tls handshake timeout",
		"no such host",
		"stream error",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

func statusCodeFromMessage(msg string) int {
	msg = strings.ToLower(msg)
	for _, code := range []int{429, 404, 401, 403, 503, 502, 500, 504, 408} {
		if strings.Contains(msg, strconv.Itoa(code)) {
			return code
		}
	}
	return 0
}

func BackoffDelay(policy config.APIResiliencePolicy, attempt int, retryAfter time.Duration, rng *rand.Rand) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exp := policy.BaseDelay
	for i := 1; i < attempt; i++ {
		exp *= 2
		if exp >= policy.MaxDelay {
			exp = policy.MaxDelay
			break
		}
	}
	if exp > policy.MaxDelay {
		exp = policy.MaxDelay
	}
	wait := exp
	if retryAfter > wait {
		wait = retryAfter
	}
	if wait > policy.MaxDelay {
		wait = policy.MaxDelay
	}
	if !policy.Jitter || rng == nil {
		return wait
	}
	j := wait / 2
	if j <= 0 {
		return wait
	}
	return wait/2 + time.Duration(rng.Int63n(int64(j+1)))
}

type CircuitRegistry struct {
	mu    sync.Mutex
	open  map[string]time.Time
}

func NewCircuitRegistry() *CircuitRegistry {
	return &CircuitRegistry{open: make(map[string]time.Time)}
}

var defaultCircuits = NewCircuitRegistry()

func (c *CircuitRegistry) IsOpen(hostKey string) bool {
	if c == nil || hostKey == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	until, ok := c.open[hostKey]
	if !ok {
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	delete(c.open, hostKey)
	return false
}

func (c *CircuitRegistry) Trip(hostKey string, openFor time.Duration) {
	if c == nil || hostKey == "" || openFor <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.open[hostKey] = time.Now().Add(openFor)
}

func (c *CircuitRegistry) Reset(hostKey string) {
	if c == nil || hostKey == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.open, hostKey)
}

func NewResilientHTTPClient(policy config.APIResiliencePolicy) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: policy.ConnectTimeout,
		}).DialContext,
		ResponseHeaderTimeout: policy.ConnectTimeout,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}

func logAPIRetry(hostKey, protocol string, attempt, max int, status int, wait time.Duration, err error) {
	logging.Log(logging.WARNING_LOG_LEVEL, "LLM API retry", logging.LogOptions{Params: map[string]any{
		"provider_host": hostKey,
		"protocol":      protocol,
		"attempt":       attempt,
		"max_retries":   max,
		"status_code":   status,
		"retryable":     true,
		"wait_ms":       wait.Milliseconds(),
		"err":           err.Error(),
	}})
}

func logAPIFailure(hostKey, protocol string, attempt, max int, status int, err error) {
	logging.Log(logging.ERROR_LOG_LEVEL, "LLM API request failed", logging.LogOptions{Params: map[string]any{
		"provider_host": hostKey,
		"protocol":      protocol,
		"attempt":       attempt,
		"max_retries":   max,
		"status_code":   status,
		"retryable":     false,
		"err":           err.Error(),
	}})
}

func RetryMessage(attempt, max int, err error, wait time.Duration) string {
	detail := "request failed"
	if err != nil {
		detail = err.Error()
		if len(detail) > 120 {
			detail = detail[:120] + "..."
		}
	}
	next := attempt + 1
	if next > max {
		next = max
	}
	ws := wait.Round(100 * time.Millisecond).String()
	return fmt.Sprintf("API error (%s), retrying turn %d/%d in %s...", detail, next, max, ws)
}

func logCircuitTrip(hostKey string, openFor time.Duration) {
	logging.Log(logging.ERROR_LOG_LEVEL, "LLM API circuit open", logging.LogOptions{Params: map[string]any{
		"provider_host":   hostKey,
		"circuit_open_ms": openFor.Milliseconds(),
	}})
}
