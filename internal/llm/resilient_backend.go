package llm

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type ResilientBackend struct {
	Inner         CompletionBackend
	HostKey       string
	Policy        config.APIResiliencePolicy
	Circuits      *CircuitRegistry
	SidecarRevive func(context.Context, error)
	// BeforeRetry prepares network state after backoff, before a new attempt.
	BeforeRetry func()
	rng         *rand.Rand
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

func (b *ResilientBackend) runWithRetry(ctx context.Context, opts StreamOpts, op func(context.Context) error) error {
	max := b.maxAttempts()
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, trace := traceAPIAttempt(ctx)
		lastErr = op(attemptCtx)
		trace.duration = time.Since(trace.start)
		if err := ctx.Err(); err != nil {
			return err
		}
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
			logAPIFailure(b.HostKey, string(b.Protocol()), attempt, max, status, lastErr, trace)
			if attempt <= 1 {
				return lastErr
			}
			return fmt.Errorf("after %d attempt(s): %w", attempt, lastErr)
		}
		if b.SidecarRevive != nil {
			b.SidecarRevive(ctx, lastErr)
		}
		wait := BackoffDelay(b.Policy, attempt, retryAfter, b.rng)
		if opts.OnRetry != nil {
			opts.OnRetry(attempt, max, lastErr, wait)
		}
		logAPIRetry(b.HostKey, string(b.Protocol()), attempt, max, status, wait, lastErr, trace)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.BeforeRetry != nil {
			b.BeforeRetry()
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
	err := b.runWithRetry(ctx, opts, func(attemptCtx context.Context) error {
		var reasoningStarted, reasoningEnded time.Time
		attemptOpts := opts
		attemptOpts.OnDelta = func(channel, text string) {
			if channel == "content" && strings.TrimSpace(text) != "" && reasoningEnded.IsZero() {
				reasoningEnded = time.Now()
			}
			if reasoningStarted.IsZero() && reasoningEnded.IsZero() && channel == "reasoning" && strings.TrimSpace(text) != "" {
				reasoningStarted = time.Now()
			}
			if opts.OnDelta != nil {
				opts.OnDelta(channel, text)
			}
		}
		var err error
		result, err = b.Inner.StreamTurn(attemptCtx, req, contentOut, attemptOpts)
		if err == nil && !reasoningStarted.IsZero() {
			if reasoningEnded.IsZero() {
				reasoningEnded = time.Now()
			}
			result.Usage.ReasoningStreamSecs = reasoningEnded.Sub(reasoningStarted).Seconds()
		}
		return err
	})
	return result, err
}

func (b *ResilientBackend) StreamText(ctx context.Context, req SimpleCompletionRequest, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error) {
	var text string
	var usage UsageStats
	err := b.runWithRetry(ctx, opts, func(attemptCtx context.Context) error {
		var err error
		text, usage, err = b.Inner.StreamText(attemptCtx, req, contentOut, opts)
		return err
	})
	return text, usage, err
}

func (b *ResilientBackend) CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error) {
	var out string
	cctx, cancel := b.ctxWithReadTimeout(ctx)
	defer cancel()
	opts := StreamOpts{}
	err := b.runWithRetry(cctx, opts, func(attemptCtx context.Context) error {
		var err error
		out, err = b.Inner.CompleteText(attemptCtx, req)
		return err
	})
	return out, err
}

func (b *ResilientBackend) ListModels(ctx context.Context) ([]string, error) {
	var out []string
	cctx, cancel := b.ctxWithReadTimeout(ctx)
	defer cancel()
	opts := StreamOpts{}
	err := b.runWithRetry(cctx, opts, func(attemptCtx context.Context) error {
		var err error
		out, err = b.Inner.ListModels(attemptCtx)
		return err
	})
	return out, err
}
