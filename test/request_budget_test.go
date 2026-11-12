package test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/requestbudget"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
)

type budgetFetcher func(context.Context, webfetch.Request) (webfetch.Result, error)

func (f budgetFetcher) Fetch(ctx context.Context, req webfetch.Request) (webfetch.Result, error) {
	return f(ctx, req)
}

func TestWebFetchBudgetReservesFallbackTime(t *testing.T) {
	first, second := 0, 0
	router := webfetch.NewRouter(map[string]webfetch.Fetcher{
		webfetch.ExaAdapterName: budgetFetcher(func(ctx context.Context, _ webfetch.Request) (webfetch.Result, error) {
			first++
			<-ctx.Done()
			return webfetch.Result{}, ctx.Err()
		}),
		webfetch.ParallelAdapterName: budgetFetcher(func(ctx context.Context, _ webfetch.Request) (webfetch.Result, error) {
			second++
			if err := ctx.Err(); err != nil {
				return webfetch.Result{}, err
			}
			return webfetch.Result{Markdown: "fallback"}, nil
		}),
	}, webfetch.RouterOptions{Store: search.NewMemoryBalanceStore(search.BalanceState{})})
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	result, err := router.Fetch(ctx, webfetch.Request{URL: "https://example.com"})
	if err != nil || result.Markdown != "fallback" || first != 1 || second != 1 || ctx.Err() != nil {
		t.Fatalf("fallback lost its budget: result=%#v err=%v calls=%d,%d", result, err, first, second)
	}
	attempts := result.Metadata.Attempts
	if len(attempts) != 2 || attempts[0].ErrorKind != "timeout" || !attempts[0].Retryable || attempts[0].BudgetMs <= 0 {
		t.Fatalf("missing attempt classification: %#v", attempts)
	}
}

func TestWebFetchBudgetReclaimsUnusedTime(t *testing.T) {
	var budgets []time.Duration
	fetch := budgetFetcher(func(ctx context.Context, _ webfetch.Request) (webfetch.Result, error) {
		deadline, _ := ctx.Deadline()
		budgets = append(budgets, time.Until(deadline))
		return webfetch.Result{}, errors.New("provider unavailable")
	})
	router := webfetch.NewRouter(map[string]webfetch.Fetcher{webfetch.ExaAdapterName: fetch, webfetch.ParallelAdapterName: fetch}, webfetch.RouterOptions{Store: search.NewMemoryBalanceStore(search.BalanceState{}), Fallback: fetch})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = router.Fetch(ctx, webfetch.Request{URL: "https://example.com"})
	if len(budgets) != 3 || budgets[1] <= budgets[0] || budgets[2] <= budgets[1] {
		t.Fatalf("unused time not reclaimed: %v", budgets)
	}
}

func TestWebFetchCancellationStopsFallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	fetch := budgetFetcher(func(_ context.Context, _ webfetch.Request) (webfetch.Result, error) {
		calls++
		cancel()
		return webfetch.Result{}, context.Canceled
	})
	router := webfetch.NewRouter(map[string]webfetch.Fetcher{webfetch.ExaAdapterName: fetch, webfetch.ParallelAdapterName: fetch}, webfetch.RouterOptions{Store: search.NewMemoryBalanceStore(search.BalanceState{})})
	_, err := router.Fetch(ctx, webfetch.Request{URL: "https://example.com"})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancelled fetch continued: calls=%d err=%v", calls, err)
	}
}

func TestRequestBudgetRejectsUnaffordableBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := requestbudget.Wait(ctx, 10*time.Second); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("backoff must not outlive budget: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("backoff check cancelled its parent")
	}
}

type budgetBackend struct {
	mockCompletionBackend
	calls int
}

func (b *budgetBackend) StreamTurn(ctx context.Context, _ llm.TurnRequest, _ io.Writer, _ llm.StreamOpts) (llm.AssistantTurnResult, error) {
	b.calls++
	if b.calls == 1 {
		<-ctx.Done()
		return llm.AssistantTurnResult{}, ctx.Err()
	}
	return llm.AssistantTurnResult{Content: "recovered"}, nil
}

func TestLLMRetryBudgetReservesNextAttempt(t *testing.T) {
	inner := &budgetBackend{}
	backend := llm.NewResilientBackend(inner, "budget.test", config.APIResiliencePolicy{MaxRetries: 2, ReadTimeout: 500 * time.Millisecond, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}, llm.NewCircuitRegistry())
	result, err := backend.StreamTurn(context.Background(), llm.TurnRequest{}, io.Discard, llm.StreamOpts{})
	if err != nil || result.Content != "recovered" || inner.calls != 2 {
		t.Fatalf("retry starved: calls=%d result=%#v err=%v", inner.calls, result, err)
	}
}

func TestGOAWAYIsClassifiedTransient(t *testing.T) {
	kind, _, _ := llm.ClassifyAPIError(errors.New("http2: server sent GOAWAY and closed the connection"), false)
	if kind != llm.ErrorRetryable {
		t.Fatalf("GOAWAY should be transient: %v", kind)
	}
}

func TestDirectFetchPreservesRateLimitWhenBudgetIsInsufficient(t *testing.T) {
	webfetch.ResetSharedClientForTest()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "10")
		http.Error(w, "busy", http.StatusTooManyRequests)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := webfetch.FetchURL(ctx, server.URL, 30, nil)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "429") || hits.Load() != 1 {
		t.Fatalf("rate limit was lost or retried early: calls=%d err=%v", hits.Load(), err)
	}
}

func TestDirectFetchDoesNotRetryPermanentHTTPFailures(t *testing.T) {
	webfetch.ResetSharedClientForTest()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	_, err := webfetch.FetchURL(context.Background(), server.URL, 1, nil)
	if err == nil || hits.Load() != 1 {
		t.Fatalf("permanent failure was retried: calls=%d err=%v", hits.Load(), err)
	}
}

func TestNetworkStreamIDsAreNotHTTPStatusCodes(t *testing.T) {
	for _, id := range []string{"401", "403", "404", "429", "500"} {
		kind, status, _ := llm.ClassifyAPIError(errors.New("http2: server sent GOAWAY; LastStreamID="+id+", ErrCode=NO_ERROR"), false)
		if kind != llm.ErrorRetryable || status != 0 {
			t.Fatalf("stream ID %s misclassified as HTTP %d (%v)", id, status, kind)
		}
	}
	for _, message := range []string{"API HTTP 429: limited", "status_code=429", "429 Too Many Requests"} {
		kind, status, _ := llm.ClassifyAPIError(errors.New(message), false)
		if kind != llm.ErrorRetryable || status != 429 {
			t.Fatalf("lost real HTTP status: %q: %v %d", message, kind, status)
		}
	}
}

type rateBudgetBackend struct {
	mockCompletionBackend
	calls int
}

func (b *rateBudgetBackend) StreamTurn(context.Context, llm.TurnRequest, io.Writer, llm.StreamOpts) (llm.AssistantTurnResult, error) {
	b.calls++
	return llm.AssistantTurnResult{}, llm.NewProviderHTTPError(http.StatusTooManyRequests, "limited", time.Second)
}

func TestLLMBudgetDoesNotShortenRetryAfter(t *testing.T) {
	inner := &rateBudgetBackend{}
	backend := llm.NewResilientBackend(inner, "rate-budget.test", config.APIResiliencePolicy{MaxRetries: 3, ReadTimeout: 200 * time.Millisecond, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}, llm.NewCircuitRegistry())
	_, err := backend.StreamTurn(context.Background(), llm.TurnRequest{}, io.Discard, llm.StreamOpts{})
	if !errors.Is(err, context.DeadlineExceeded) || inner.calls != 1 {
		t.Fatalf("retry-after was shortened: calls=%d err=%v", inner.calls, err)
	}
	var upstream *transport.ProviderHTTPError
	if !errors.As(err, &upstream) || upstream.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("rate-limit cause was lost: %v", err)
	}
}
