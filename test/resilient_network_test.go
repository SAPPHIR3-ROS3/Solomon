package test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestRetryPreparesNetworkAfterStreamFailure(t *testing.T) {
	b := llm.NewResilientBackend(nil, "network-test", config.APIResiliencePolicy{MaxRetries: 2}, llm.NewCircuitRegistry())
	prepared, attempts := 0, 0
	b.BeforeRetry = func() { prepared++ }
	b.Inner = retryNetworkBackend{complete: func() error {
		attempts++
		if attempts == 1 {
			return io.ErrUnexpectedEOF
		}
		if prepared != 1 {
			t.Fatal("retry ran before network preparation")
		}
		return nil
	}}
	_, err := b.CompleteText(context.Background(), llm.SimpleCompletionRequest{})
	if err != nil || attempts != 2 || prepared != 1 {
		t.Fatalf("attempts=%d prepared=%d err=%v", attempts, prepared, err)
	}
}

func TestCancelledRetryDoesNotPrepareNetwork(t *testing.T) {
	b := llm.NewResilientBackend(nil, "network-test", config.APIResiliencePolicy{MaxRetries: 2}, llm.NewCircuitRegistry())
	b.BeforeRetry = func() { t.Fatal("prepared network for cancelled request") }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.Inner = retryNetworkBackend{complete: func() error { cancel(); return io.ErrUnexpectedEOF }}
	_, err := b.CompleteText(ctx, llm.SimpleCompletionRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

type retryNetworkBackend struct {
	llm.CompletionBackend
	complete func() error
}

func (b retryNetworkBackend) CompleteText(context.Context, llm.SimpleCompletionRequest) (string, error) {
	return "", b.complete()
}
func (b retryNetworkBackend) Protocol() llm.Protocol { return llm.ProtocolOpenAI }
