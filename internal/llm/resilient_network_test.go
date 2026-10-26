package llm

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func TestRetryPreparesNetworkAfterStreamFailure(t *testing.T) {
	logging.LogInit(logging.ERROR_LOG_LEVEL)
	b := NewResilientBackend(nil, "network-test", config.APIResiliencePolicy{MaxRetries: 2}, NewCircuitRegistry())
	prepared, attempts := 0, 0
	b.BeforeRetry = func() { prepared++ }
	err := b.runWithRetry(context.Background(), StreamOpts{}, func() error {
		attempts++
		if attempts == 1 {
			return io.ErrUnexpectedEOF
		}
		if prepared != 1 {
			t.Fatal("retry ran before network preparation")
		}
		return nil
	})
	if err != nil || attempts != 2 || prepared != 1 {
		t.Fatalf("attempts=%d prepared=%d err=%v", attempts, prepared, err)
	}
}

func TestCancelledRetryDoesNotPrepareNetwork(t *testing.T) {
	b := NewResilientBackend(nil, "network-test", config.APIResiliencePolicy{MaxRetries: 2}, NewCircuitRegistry())
	b.BeforeRetry = func() { t.Fatal("prepared network for cancelled request") }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := b.runWithRetry(ctx, StreamOpts{}, func() error { cancel(); return io.ErrUnexpectedEOF })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
