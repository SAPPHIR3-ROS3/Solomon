package test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

type mockCompletionBackend struct {
	protocol    llm.Protocol
	streamCalls atomic.Int32
	failUntil   int32
}

func (m *mockCompletionBackend) Protocol() llm.Protocol { return m.protocol }

func (m *mockCompletionBackend) StreamTurn(ctx context.Context, req llm.TurnRequest, contentOut io.Writer, opts llm.StreamOpts) (llm.AssistantTurnResult, error) {
	n := m.streamCalls.Add(1)
	if n <= m.failUntil {
		return llm.AssistantTurnResult{}, llm.NewProviderHTTPError(503, "unavailable", 0)
	}
	return llm.AssistantTurnResult{Content: "ok"}, nil
}

func (m *mockCompletionBackend) StreamText(ctx context.Context, req llm.SimpleCompletionRequest, contentOut io.Writer, opts llm.StreamOpts) (string, llm.UsageStats, error) {
	return "", llm.UsageStats{}, errors.New("not implemented")
}

func (m *mockCompletionBackend) CompleteText(ctx context.Context, req llm.SimpleCompletionRequest) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockCompletionBackend) ListModels(ctx context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

func TestResilientBackend_StreamTurn_RetriesThenSucceeds(t *testing.T) {
	inner := &mockCompletionBackend{protocol: llm.ProtocolAnthropic, failUntil: 2}
	circuits := llm.NewCircuitRegistry()
	policy := config.APIResiliencePolicy{
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   5 * time.Millisecond,
		Jitter:     false,
		CircuitOpen: time.Minute,
	}
	rb := llm.NewResilientBackend(inner, "mock.test", policy, circuits)
	var retries int
	opts := llm.StreamOpts{
		OnRetry: func(attempt, max int, err error, wait time.Duration) {
			retries++
		},
	}
	turn, err := rb.StreamTurn(context.Background(), llm.TurnRequest{
		Model:    "m",
		Messages: []chatstore.Message{{Role: "user", Content: "hi"}},
	}, io.Discard, opts)
	if err != nil {
		t.Fatal(err)
	}
	if turn.Content != "ok" {
		t.Fatalf("content: %q", turn.Content)
	}
	if inner.streamCalls.Load() != 3 {
		t.Fatalf("calls: got %d want 3", inner.streamCalls.Load())
	}
	if retries != 2 {
		t.Fatalf("onRetry: got %d want 2", retries)
	}
}

func TestResilientBackend_StreamTurn_NoRetryOn401(t *testing.T) {
	policy := config.APIResiliencePolicy{MaxRetries: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, Jitter: false, CircuitOpen: time.Minute}
	rb := llm.NewResilientBackend(&failing401Backend{}, "mock.test", policy, llm.NewCircuitRegistry())
	_, err := rb.StreamTurn(context.Background(), llm.TurnRequest{Model: "m"}, io.Discard, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResilientBackend_StreamTurn_singleAttemptNoAfterPrefix(t *testing.T) {
	policy := config.APIResiliencePolicy{MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, Jitter: false, CircuitOpen: time.Minute}
	rb := llm.NewResilientBackend(&failingMalformedLegacyBackend{}, "mock.test", policy, llm.NewCircuitRegistry())
	_, err := rb.StreamTurn(context.Background(), llm.TurnRequest{Model: "m"}, io.Discard, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "after 1 attempt(s)") {
		t.Fatalf("single permanent failure should not add retry prefix: %q", err.Error())
	}
	if !errors.Is(err, tooling.ErrMalformedLegacyTool) {
		t.Fatalf("want malformed legacy, got %v", err)
	}
}

type failingMalformedLegacyBackend struct{}

func (f *failingMalformedLegacyBackend) Protocol() llm.Protocol { return llm.ProtocolOpenAI }

func (f *failingMalformedLegacyBackend) StreamTurn(ctx context.Context, req llm.TurnRequest, contentOut io.Writer, opts llm.StreamOpts) (llm.AssistantTurnResult, error) {
	return llm.AssistantTurnResult{}, fmt.Errorf("%w: unexpected content outside tool tags: %q", tooling.ErrMalformedLegacyTool, "oops")
}

func (f *failingMalformedLegacyBackend) StreamText(ctx context.Context, req llm.SimpleCompletionRequest, contentOut io.Writer, opts llm.StreamOpts) (string, llm.UsageStats, error) {
	return "", llm.UsageStats{}, errors.New("not implemented")
}

func (f *failingMalformedLegacyBackend) CompleteText(ctx context.Context, req llm.SimpleCompletionRequest) (string, error) {
	return "", errors.New("not implemented")
}

func (f *failingMalformedLegacyBackend) ListModels(ctx context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

type failing401Backend struct{}

func (f *failing401Backend) Protocol() llm.Protocol { return llm.ProtocolOpenAI }

func (f *failing401Backend) StreamTurn(ctx context.Context, req llm.TurnRequest, contentOut io.Writer, opts llm.StreamOpts) (llm.AssistantTurnResult, error) {
	return llm.AssistantTurnResult{}, llm.NewProviderHTTPError(401, "unauthorized", 0)
}

func (f *failing401Backend) StreamText(ctx context.Context, req llm.SimpleCompletionRequest, contentOut io.Writer, opts llm.StreamOpts) (string, llm.UsageStats, error) {
	return "", llm.UsageStats{}, errors.New("not implemented")
}

func (f *failing401Backend) CompleteText(ctx context.Context, req llm.SimpleCompletionRequest) (string, error) {
	return "", errors.New("not implemented")
}

func (f *failing401Backend) ListModels(ctx context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

func TestResilientBackend_CircuitOpenAfterExhaustedRetries(t *testing.T) {
	inner := &mockCompletionBackend{protocol: llm.ProtocolAnthropic, failUntil: 99}
	circuits := llm.NewCircuitRegistry()
	policy := config.APIResiliencePolicy{
		MaxRetries:  2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
		Jitter:      false,
		CircuitOpen: time.Minute,
	}
	host := "circuit.example.com"
	rb := llm.NewResilientBackend(inner, host, policy, circuits)
	_, err := rb.StreamTurn(context.Background(), llm.TurnRequest{Model: "m"}, io.Discard, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !circuits.IsOpen(host) {
		t.Fatal("expected circuit open")
	}
	callsBefore := inner.streamCalls.Load()
	_, err = rb.StreamTurn(context.Background(), llm.TurnRequest{Model: "m"}, io.Discard, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error on probe while provider still failing")
	}
	if inner.streamCalls.Load() <= callsBefore {
		t.Fatal("expected probe call through open circuit")
	}
	if !circuits.IsOpen(host) {
		t.Fatal("circuit should stay open after failed probe")
	}
}

func TestResilientBackend_CircuitResetOnFirstSuccess(t *testing.T) {
	inner := &mockCompletionBackend{protocol: llm.ProtocolAnthropic, failUntil: 0}
	circuits := llm.NewCircuitRegistry()
	host := "reset-on-success.example.com"
	circuits.Trip(host, time.Hour)
	if !circuits.IsOpen(host) {
		t.Fatal("expected circuit open before test")
	}
	policy := config.APIResiliencePolicy{
		MaxRetries:  3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
		Jitter:      false,
		CircuitOpen: time.Hour,
	}
	rb := llm.NewResilientBackend(inner, host, policy, circuits)
	turn, err := rb.StreamTurn(context.Background(), llm.TurnRequest{Model: "m"}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Content != "ok" {
		t.Fatalf("content: %q", turn.Content)
	}
	if circuits.IsOpen(host) {
		t.Fatal("circuit should reset on successful turn")
	}
}
