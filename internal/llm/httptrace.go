package llm

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type apiHTTPTrace struct {
	requestID string
	duration  time.Duration
	start     time.Time
	mu        sync.Mutex
	events    []string
	omitted   int
}

func traceAPIAttempt(ctx context.Context) (context.Context, *apiHTTPTrace) {
	trace := &apiHTTPTrace{start: time.Now(), requestID: logging.RequestID(ctx)}
	hooks := &httptrace.ClientTrace{
		GetConn:           func(string) { trace.record("get_connection", nil) },
		DNSStart:          func(httptrace.DNSStartInfo) { trace.record("dns_start", nil) },
		DNSDone:           func(info httptrace.DNSDoneInfo) { trace.record("dns_done", info.Err) },
		ConnectStart:      func(_, _ string) { trace.record("tcp_start", nil) },
		ConnectDone:       func(_, _ string, err error) { trace.record("tcp_done", err) },
		TLSHandshakeStart: func() { trace.record("tls_start", nil) },
		TLSHandshakeDone:  func(_ tls.ConnectionState, err error) { trace.record("tls_done", err) },
		GotConn: func(info httptrace.GotConnInfo) {
			trace.record(fmt.Sprintf("connection_acquired(reused=%t)", info.Reused), nil)
		},
		WroteRequest:         func(info httptrace.WroteRequestInfo) { trace.record("request_written", info.Err) },
		GotFirstResponseByte: func() { trace.record("first_response_byte", nil) },
	}
	return httptrace.WithClientTrace(ctx, hooks), trace
}

func (t *apiHTTPTrace) record(event string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.events) >= 32 {
		t.omitted++
		t.events = t.events[1:]
	}
	if err != nil {
		event += fmt.Sprintf("(error=%T)", err)
	}
	t.events = append(t.events, fmt.Sprintf("%dms:%s", time.Since(t.start).Milliseconds(), event))
}

func (t *apiHTTPTrace) addFields(params map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	params["attempt_duration_ms"] = t.duration.Milliseconds()
	if t.requestID != "" {
		params["request_id"] = t.requestID
	}
	if len(t.events) > 0 {
		params["http_trace"] = strings.Join(t.events, " ")
	}
	if t.omitted > 0 {
		params["http_trace_omitted"] = t.omitted
	}
}
