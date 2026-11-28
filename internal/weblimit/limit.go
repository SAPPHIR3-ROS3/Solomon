package weblimit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

var UnknownReset = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

type Suspended struct {
	Until time.Time
}

func (e *Suspended) Error() string {
	if e.Until.Equal(UnknownReset) {
		return "web backend rate limit: requests suspended for this chat (provider reset unknown)"
	}
	return fmt.Sprintf("web backend rate limit: requests suspended until %s", e.Until.Format(time.RFC3339Nano))
}

func IsSuspended(err error) bool {
	var suspended *Suspended
	return errors.As(err, &suspended)
}

type Recorded struct {
	Err error
}

func (e *Recorded) Error() string { return e.Err.Error() }
func (e *Recorded) Unwrap() error { return e.Err }

func Reset(err error, now time.Time) (time.Time, bool) {
	if err == nil {
		return time.Time{}, false
	}
	_, reset, limited := llm.RateLimitDetails(err, now)
	var httpErr *transport.ProviderHTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusTooManyRequests && httpErr.RetryAfter > 0 {
		if retryAt := now.Add(httpErr.RetryAfter); retryAt.After(reset) {
			reset = retryAt
		}
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"rate_limit", "too many requests", "status code 429", "status 429"} {
		limited = limited || strings.Contains(message, marker)
	}
	if limited && reset.IsZero() {
		reset = UnknownReset
	}
	return reset, limited
}

func HTTPError(retryAfter string) error {
	response := &http.Response{Header: http.Header{"Retry-After": []string{retryAfter}}}
	return transport.NewProviderHTTPError(http.StatusTooManyRequests, "", transport.ParseRetryAfterHeader(response))
}

type MCPAvailability interface {
	CheckInternalRateLimit(context.Context, string) error
}

type BrowserStore interface {
	CheckBrowserRateLimit(context.Context, string) error
	RecordBrowserRateLimit(context.Context, string, time.Time)
}

type BrowserLimits struct {
	Store  BrowserStore
	resets sync.Map
}

func (l *BrowserLimits) Check(ctx context.Context, origin string) error {
	if l.Store != nil {
		return l.Store.CheckBrowserRateLimit(ctx, origin)
	}
	if until, ok := l.resets.Load(origin); ok && time.Now().Before(until.(time.Time)) {
		return &Suspended{Until: until.(time.Time)}
	}
	return nil
}

func (l *BrowserLimits) Record(ctx context.Context, origin, retryAfter string) error {
	err := HTTPError(retryAfter)
	until, _ := Reset(err, time.Now())
	if l.Store != nil {
		l.Store.RecordBrowserRateLimit(ctx, origin, until)
	} else {
		l.resets.Store(origin, until)
	}
	return err
}
