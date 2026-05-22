package test

import (
	"errors"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
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
