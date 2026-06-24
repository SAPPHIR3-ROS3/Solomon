package test

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestClassifyAPIError_RetryableAndPermanent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err    error
		open   bool
		want   llm.ErrorClass
		status int
	}{
		{llm.NewProviderHTTPError(429, "rate", 0), false, llm.ErrorRetryable, 429},
		{llm.NewProviderHTTPError(503, "unavailable", 0), false, llm.ErrorRetryable, 503},
		{llm.NewProviderHTTPError(401, "auth", 0), false, llm.ErrorPermanent, 401},
		{llm.ErrStreamAccumulatorRejected, false, llm.ErrorPermanent, 0},
		{errors.New("connection reset by peer"), false, llm.ErrorRetryable, 0},
		{nil, true, llm.ErrorCircuitOpen, 0},
	}
	for _, tc := range cases {
		got, status, _ := llm.ClassifyAPIError(tc.err, tc.open)
		if got != tc.want {
			t.Fatalf("err %v open=%v: class got %v want %v", tc.err, tc.open, got, tc.want)
		}
		if status != tc.status {
			t.Fatalf("err %v: status got %d want %d", tc.err, status, tc.status)
		}
	}
}

func TestBackoffDelay_CappedAndRetryAfter(t *testing.T) {
	policy := config.APIResiliencePolicy{
		BaseDelay:  time.Second,
		MaxDelay:   5 * time.Second,
		Jitter:     false,
	}
	rng := rand.New(rand.NewSource(1))
	wait := llm.BackoffDelay(policy, 2, 0, rng)
	if wait != 2*time.Second {
		t.Fatalf("attempt 2: got %v want 2s", wait)
	}
	wait = llm.BackoffDelay(policy, 1, 10*time.Second, rng)
	if wait != 5*time.Second {
		t.Fatalf("retry-after cap: got %v want 5s", wait)
	}
}

func TestUserFacingAPIError_anthropicRateLimit(t *testing.T) {
	t.Parallel()
	raw := `after 3 attempt(s): API HTTP 429: {"type":"error","error":{"type":"rate_limit_error","message":"Error"},"request_id":"req_011CcFQoxsGHhq6pAfiourBe"}`
	got := llm.UserFacingAPIError(errors.New(raw))
	for _, want := range []string{
		"summary: rate limit reached",
		"attempts: 3",
		"HTTP: 429",
		"type: rate_limit_error",
		"message: too many requests",
		"request_id: req_011CcFQoxsGHhq6pAfiourBe",
		"hint:",
		"sk-ant-oat",
		"paused this provider",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{") {
		t.Fatalf("expected no raw JSON, got:\n%s", got)
	}
}

func TestUserFacingAPIError_anthropicModelNotFound(t *testing.T) {
	t.Parallel()
	raw := `API HTTP 404: {"type":"error","error":{"type":"not_found_error","message":"model: claude-sonnet-4-20250514"},"request_id":"req_011CcFPUd4zYMvr59S1Zw8CZ"}`
	got := llm.UserFacingAPIError(errors.New(raw))
	for _, want := range []string{
		"summary: model not found",
		"HTTP: 404",
		"type: not_found_error",
		"model: claude-sonnet-4-20250514",
		"retired",
		"claude-sonnet-4-6",
		"/models",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestUserFacingAPIError_usageLimitReached(t *testing.T) {
	t.Parallel()
	raw := `after 3 attempt(s): POST "https://chatgpt.com/backend-api/codex/v1/chat/completions": 429 Too Many Requests {"type":"usage_limit_reached","message":"The usage limit has been reached","plan_type":"free","resets_at":1779966197,"eligible_promo":null,"resets_in_seconds":133272}`
	got := llm.UserFacingAPIError(errors.New(raw))
	for _, want := range []string{
		"attempts: 3",
		"HTTP: 429",
		"type: usage_limit_reached",
		"message: The usage limit has been reached",
		"plan: free",
		"reset:",
		"(in 133272 seconds)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{") {
		t.Fatalf("expected no raw JSON, got:\n%s", got)
	}
}

func TestUserFacingAPIError_providerHTTPError(t *testing.T) {
	t.Parallel()
	err := llm.NewProviderHTTPError(429, `{"type":"usage_limit_reached","message":"limit"}`, 0)
	got := llm.UserFacingAPIError(err)
	if !strings.Contains(got, "type: usage_limit_reached") || !strings.Contains(got, "HTTP: 429") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestUserFacingAPIError_nestedProxyError(t *testing.T) {
	t.Parallel()
	raw := `models API: 500 Internal Server Error: {"error":{"message":"Network request failed","type":"proxy_error"}}`
	got := llm.UserFacingAPIError(errors.New(raw))
	for _, want := range []string{
		"source: models API",
		"HTTP: 500",
		"type: proxy_error",
		"message: Network request failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{") {
		t.Fatalf("expected no raw JSON, got:\n%s", got)
	}
}

func TestUserFacingAPIError_httpRequestError(t *testing.T) {
	t.Parallel()
	raw := `Get "https://openrouter.ai/api/v1/models": dial tcp: lookup openrouter.ai: no such host`
	got := llm.UserFacingAPIError(errors.New(raw))
	for _, want := range []string{
		"request: GET https://openrouter.ai/api/v1/models",
		"error: dial tcp: lookup openrouter.ai: no such host",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestIsLocalEndpoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want bool
	}{
		{"http://127.0.0.1:8766/v1/", true},
		{"http://localhost:8080", true},
		{"http://[::1]:8080", true},
		{"http://api.openrouter.ai/v1", false},
		{"https://example.com", false},
	}
	for _, tc := range cases {
		if got := config.IsLocalEndpoint(tc.raw); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.raw, got, tc.want)
		}
	}
}

func TestFormatOfflineNotice(t *testing.T) {
	t.Parallel()
	cfg := &config.Root{
		Providers: map[string]*config.Provider{
			"Cursor API": {BaseURL: "http://127.0.0.1:8766/v1/"},
			"OpenRouter": {BaseURL: "https://openrouter.ai/api/v1"},
		},
	}
	got := commands.FormatOfflineNotice(cfg)
	for _, want := range []string{
		"No internet connection detected.",
		"Until connectivity is restored",
		"- web search",
		"- remote providers: OpenRouter",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Cursor API") {
		t.Fatalf("local provider should not be listed:\n%s", got)
	}
}

func TestStartupWasOfflineRetryOnce(t *testing.T) {
	t.Parallel()
	commands.SetStartupOfflineForTest(true)
	if !commands.StartupWasOffline() {
		t.Fatal("expected offline flag")
	}
	commands.ClearStartupOfflineForTest()
	if commands.StartupWasOffline() {
		t.Fatal("expected offline flag cleared")
	}
}

func TestInternetReachable_probe(t *testing.T) {
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer okSrv.Close()
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failSrv.Close()

	commands.SetInternetProbeURLsForTest([]string{failSrv.URL, okSrv.URL})
	t.Cleanup(commands.ResetInternetProbeURLsForTest)

	if !commands.InternetReachable(context.Background(), nil) {
		t.Fatal("expected reachable when a probe succeeds")
	}

	commands.SetInternetProbeURLsForTest([]string{failSrv.URL})
	if commands.InternetReachable(context.Background(), nil) {
		t.Fatal("expected unreachable when all probes fail")
	}
}

func TestInternetReachable_unreachableHost(t *testing.T) {
	commands.SetInternetProbeURLsForTest([]string{"http://127.0.0.1:1"})
	t.Cleanup(commands.ResetInternetProbeURLsForTest)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if commands.InternetReachable(ctx, nil) {
		t.Fatal("expected unreachable for dead endpoint")
	}
}

func TestCircuitRegistry_ResetClosesBeforeExpiry(t *testing.T) {
	c := llm.NewCircuitRegistry()
	host := "api.example.com"
	c.Trip(host, time.Hour)
	if !c.IsOpen(host) {
		t.Fatal("expected open")
	}
	c.Reset(host)
	if c.IsOpen(host) {
		t.Fatal("expected closed after reset")
	}
}

func TestCircuitRegistry_OpenTripExpire(t *testing.T) {
	c := llm.NewCircuitRegistry()
	host := "api.example.com"
	if c.IsOpen(host) {
		t.Fatal("expected closed")
	}
	c.Trip(host, 50*time.Millisecond)
	if !c.IsOpen(host) {
		t.Fatal("expected open")
	}
	time.Sleep(60 * time.Millisecond)
	if c.IsOpen(host) {
		t.Fatal("expected closed after expiry")
	}
}

func TestEffectiveAPIResilience_DefaultsAndClamp(t *testing.T) {
	p := config.EffectiveAPIResilience(nil)
	if p.MaxRetries != config.DefaultAPIMaxRetries {
		t.Fatalf("max_retries: got %d want %d", p.MaxRetries, config.DefaultAPIMaxRetries)
	}
	if !p.Jitter {
		t.Fatal("expected jitter on by default")
	}
	r := &config.Root{
		APIResilience: config.APIResilienceConfig{
			MaxRetries:        99,
			BaseDelayMS:       500,
			MaxDelayMS:        2000,
			ConnectTimeoutSec: 10,
			CircuitOpenSec:    30,
		},
	}
	p = config.EffectiveAPIResilience(r)
	if p.MaxRetries != 10 {
		t.Fatalf("max_retries clamp: got %d want 10", p.MaxRetries)
	}
	if p.BaseDelay != 500*time.Millisecond {
		t.Fatalf("base delay: got %v", p.BaseDelay)
	}
}

func TestHostKeyFromBaseURL(t *testing.T) {
	if got := llm.HostKeyFromBaseURL("https://api.openai.com/v1/"); got != "api.openai.com" {
		t.Fatalf("got %q", got)
	}
}

func TestClassifyAPIError_BadGateway(t *testing.T) {
	e := llm.NewProviderHTTPError(http.StatusBadGateway, "bad gateway", 0)
	if class, code, _ := llm.ClassifyAPIError(e, false); class != llm.ErrorRetryable || code != 502 {
		t.Fatalf("got class=%v code=%d", class, code)
	}
}
