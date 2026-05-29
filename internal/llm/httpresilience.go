package llm

import (
	"context"
	"encoding/json"
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

type ProviderHTTPError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *ProviderHTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("API HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API HTTP %d", e.StatusCode)
}

func NewProviderHTTPError(status int, message string, retryAfter time.Duration) *ProviderHTTPError {
	return &ProviderHTTPError{StatusCode: status, Message: strings.TrimSpace(message), RetryAfter: retryAfter}
}

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
	var phe *ProviderHTTPError
	if errors.As(err, &phe) {
		if retryableStatus(phe.StatusCode) {
			return ErrorRetryable, phe.StatusCode, phe.RetryAfter
		}
		return ErrorPermanent, phe.StatusCode, 0
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		ra := parseRetryAfterHeader(apiErr.Response)
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
	for _, code := range []int{429, 503, 502, 500, 504, 408} {
		if strings.Contains(msg, strconv.Itoa(code)) {
			return code
		}
	}
	return 0
}

func parseRetryAfterHeader(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	v := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
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

type codexAPIErrorBody struct {
	Type            string `json:"type"`
	Message         string `json:"message"`
	PlanType        string `json:"plan_type"`
	ResetsAt        int64  `json:"resets_at"`
	ResetsInSeconds int64  `json:"resets_in_seconds"`
}

func UserFacingAPIError(err error) string {
	if err == nil {
		return ""
	}
	var phe *ProviderHTTPError
	if errors.As(err, &phe) {
		return formatAPIErrorLines(0, phe.StatusCode, phe.Message)
	}
	msg := err.Error()
	attempts := 0
	if rest, n, ok := parseAfterAttemptsPrefix(msg); ok {
		attempts = n
		msg = rest
	}
	if body, ok := extractAPIErrorJSON(msg); ok {
		return formatAPIErrorLines(attempts, statusCodeFromMessage(msg), body)
	}
	if attempts > 0 {
		return fmt.Sprintf("attempts: %d\n%s", attempts, msg)
	}
	return msg
}

func parseAfterAttemptsPrefix(msg string) (rest string, attempts int, ok bool) {
	const prefix = "after "
	if !strings.HasPrefix(msg, prefix) {
		return msg, 0, false
	}
	rest = msg[len(prefix):]
	i := strings.Index(rest, " attempt(s): ")
	if i < 0 {
		return msg, 0, false
	}
	n, err := strconv.Atoi(rest[:i])
	if err != nil || n < 1 {
		return msg, 0, false
	}
	return rest[i+len(" attempt(s): "):], n, true
}

func extractAPIErrorJSON(msg string) (codexAPIErrorBody, bool) {
	idx := strings.Index(msg, "{")
	if idx < 0 {
		return codexAPIErrorBody{}, false
	}
	raw := strings.TrimSpace(msg[idx:])
	var body codexAPIErrorBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return codexAPIErrorBody{}, false
	}
	if body.Type == "" && body.Message == "" && body.ResetsAt == 0 && body.ResetsInSeconds == 0 {
		return codexAPIErrorBody{}, false
	}
	return body, true
}

func formatAPIErrorLines(attempts, statusCode int, payload any) string {
	var body codexAPIErrorBody
	switch p := payload.(type) {
	case codexAPIErrorBody:
		body = p
	case string:
		if b, ok := extractAPIErrorJSON(p); ok {
			body = b
		} else {
			body.Message = strings.TrimSpace(p)
		}
	default:
		return fmt.Sprint(payload)
	}
	var lines []string
	if attempts > 0 {
		lines = append(lines, fmt.Sprintf("attempts: %d", attempts))
	}
	if statusCode > 0 {
		lines = append(lines, fmt.Sprintf("HTTP: %d", statusCode))
	} else if s, ok := payload.(string); ok {
		if statusLine := extractHTTPStatusLine(s); statusLine != "" {
			lines = append(lines, "HTTP: "+statusLine)
		}
	}
	if body.Type != "" {
		lines = append(lines, "type: "+body.Type)
	}
	if body.Message != "" {
		lines = append(lines, "message: "+body.Message)
	}
	if body.PlanType != "" {
		lines = append(lines, "plan: "+body.PlanType)
	}
	if reset := formatAPIResetLine(body); reset != "" {
		lines = append(lines, reset)
	}
	if len(lines) == 0 {
		return fmt.Sprint(payload)
	}
	return strings.Join(lines, "\n")
}

func extractHTTPStatusLine(msg string) string {
	before := msg
	if i := strings.Index(msg, "{"); i >= 0 {
		before = strings.TrimSpace(msg[:i])
	}
	if li := strings.LastIndex(before, ": "); li >= 0 {
		tail := strings.TrimSpace(before[li+2:])
		if len(tail) >= 3 && tail[0] >= '0' && tail[0] <= '9' {
			return tail
		}
	}
	return ""
}

func formatAPIResetLine(body codexAPIErrorBody) string {
	if body.ResetsAt <= 0 && body.ResetsInSeconds <= 0 {
		return ""
	}
	var when string
	if body.ResetsAt > 0 {
		when = time.Unix(body.ResetsAt, 0).Local().Format("2006-01-02 15:04:05")
	}
	sec := body.ResetsInSeconds
	if sec <= 0 && body.ResetsAt > 0 {
		sec = int64(time.Until(time.Unix(body.ResetsAt, 0)).Seconds())
		if sec < 0 {
			sec = 0
		}
	}
	switch {
	case when != "" && sec > 0:
		return fmt.Sprintf("reset: %s (in %d seconds)", when, sec)
	case when != "":
		return "reset: " + when
	case sec > 0:
		return fmt.Sprintf("reset: in %d seconds", sec)
	default:
		return ""
	}
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
