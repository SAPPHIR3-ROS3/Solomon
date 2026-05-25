package llm

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

type ResilientBackend struct {
	Inner    CompletionBackend
	HostKey  string
	Policy   config.APIResiliencePolicy
	Circuits *CircuitRegistry
	rng      *rand.Rand
}

func NewResilientBackend(inner CompletionBackend, hostKey string, policy config.APIResiliencePolicy, circuits *CircuitRegistry) *ResilientBackend {
	if circuits == nil {
		circuits = defaultCircuits
	}
	return &ResilientBackend{
		Inner:    inner,
		HostKey:  hostKey,
		Policy:   policy,
		Circuits: circuits,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *ResilientBackend) Protocol() Protocol {
	if b.Inner == nil {
		return ""
	}
	return b.Inner.Protocol()
}

func (b *ResilientBackend) maxAttempts() int {
	n := b.Policy.MaxRetries
	if n < 1 {
		return 1
	}
	return n
}

func (b *ResilientBackend) runWithRetry(ctx context.Context, opts StreamOpts, op func() error) error {
	max := b.maxAttempts()
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		lastErr = op()
		if lastErr == nil {
			b.Circuits.Reset(b.HostKey)
			return nil
		}
		class, status, retryAfter := ClassifyAPIError(lastErr, false)
		if class == ErrorCircuitOpen {
			return lastErr
		}
		if class != ErrorRetryable || attempt >= max {
			if class == ErrorRetryable && attempt >= max {
				b.Circuits.Trip(b.HostKey, b.Policy.CircuitOpen)
				logCircuitTrip(b.HostKey, b.Policy.CircuitOpen)
			}
			logAPIFailure(b.HostKey, string(b.Protocol()), attempt, max, status, lastErr)
			if attempt <= 1 {
				return lastErr
			}
			return fmt.Errorf("after %d attempt(s): %w", attempt, lastErr)
		}
		wait := BackoffDelay(b.Policy, attempt, retryAfter, b.rng)
		if opts.OnRetry != nil {
			opts.OnRetry(attempt, max, lastErr, wait)
		}
		logAPIRetry(b.HostKey, string(b.Protocol()), attempt, max, status, wait, lastErr)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

func (b *ResilientBackend) ctxWithReadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if b.Policy.ReadTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, b.Policy.ReadTimeout)
}

func (b *ResilientBackend) StreamTurn(ctx context.Context, req TurnRequest, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error) {
	var result AssistantTurnResult
	err := b.runWithRetry(ctx, opts, func() error {
		var err error
		result, err = b.Inner.StreamTurn(ctx, req, contentOut, opts)
		return err
	})
	return result, err
}

func (b *ResilientBackend) StreamText(ctx context.Context, req SimpleCompletionRequest, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error) {
	var text string
	var usage UsageStats
	err := b.runWithRetry(ctx, opts, func() error {
		var err error
		text, usage, err = b.Inner.StreamText(ctx, req, contentOut, opts)
		return err
	})
	return text, usage, err
}

func (b *ResilientBackend) CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error) {
	var out string
	cctx, cancel := b.ctxWithReadTimeout(ctx)
	defer cancel()
	opts := StreamOpts{}
	err := b.runWithRetry(cctx, opts, func() error {
		var err error
		out, err = b.Inner.CompleteText(cctx, req)
		return err
	})
	return out, err
}

func (b *ResilientBackend) ListModels(ctx context.Context) ([]string, error) {
	var out []string
	cctx, cancel := b.ctxWithReadTimeout(ctx)
	defer cancel()
	opts := StreamOpts{}
	err := b.runWithRetry(cctx, opts, func() error {
		var err error
		out, err = b.Inner.ListModels(cctx)
		return err
	})
	return out, err
}
