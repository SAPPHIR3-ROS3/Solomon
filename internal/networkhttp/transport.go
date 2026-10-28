// Package networkhttp keeps HTTP connection pools aligned with system routing.
package networkhttp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrRouteChanged tells the owning retry loop to restart on the current route.
var ErrRouteChanged = errors.New("system network route changed")

// Transport replaces its connection pool when the system's routes change.
// New preserves active responses; NewRetrying interrupts them so the owning
// retry loop can restart. This transport never replays requests itself.
type Transport struct {
	mu                sync.Mutex
	base              *http.Transport
	current           *http.Transport
	key               string
	routeKey          func() string
	interruptOnChange bool
}

// New clones base, which must not be modified afterward.
func New(base *http.Transport) *Transport {
	clone := base.Clone()
	if clone.DialContext != nil {
		clone.DialContext = networkpolicy.WrapDial(clone.DialContext, clone.ResponseHeaderTimeout)
	} else {
		clone.DialContext = networkpolicy.DialContext
	}
	return &Transport{base: clone, routeKey: systemRouteKey}
}

// NewRetrying also interrupts active requests on route changes. Use only when
// the caller owns a retry loop and permits replaying the request.
func NewRetrying(base *http.Transport) *Transport {
	t := New(base)
	t.interruptOnChange = true
	return t
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	key := t.routeKey()
	if t.current == nil || key != t.key {
		if t.current != nil {
			t.current.CloseIdleConnections()
		}
		t.current = t.base.Clone()
		t.key = key
	}
	current := t.current
	t.mu.Unlock()
	if !t.interruptOnChange {
		return current.RoundTrip(req)
	}
	ctx, cancel := context.WithCancelCause(req.Context())
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done); cancel(nil) }) }
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.mu.Lock()
				changed := t.routeKey() != key
				t.mu.Unlock()
				if changed {
					cancel(ErrRouteChanged)
					return
				}
			}
		}
	}()
	response, err := current.RoundTrip(req.Clone(ctx))
	if err != nil {
		if errors.Is(context.Cause(ctx), ErrRouteChanged) {
			err = ErrRouteChanged
		}
		stop()
		return nil, err
	}
	response.Body = &routeBody{ReadCloser: response.Body, ctx: ctx, stop: stop}
	return response, nil
}

type routeBody struct {
	io.ReadCloser
	ctx  context.Context
	stop func()
}

func (b *routeBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		if errors.Is(context.Cause(b.ctx), ErrRouteChanged) {
			err = ErrRouteChanged
		}
		b.stop()
	}
	return n, err
}

func (b *routeBody) Close() error {
	b.stop()
	return b.ReadCloser.Close()
}

func (t *Transport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current != nil {
		t.current.CloseIdleConnections()
	}
}

// PrepareRetry discards the pool used by the failed attempt, even when a
// transient network change left the route tables unchanged. The next request
// opens a connection using the OS's current routing decision.
func (t *Transport) PrepareRetry() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current != nil {
		t.current.CloseIdleConnections()
		t.current = nil
	}
}

func systemRouteKey() string {
	// Linux route tables include interface, gateway and metric, including
	// changes where both Wi-Fi and Ethernet remain connected.
	policy, err := networkpolicy.Read()
	key := ""
	if err != nil {
		key = err.Error()
	} else {
		data, _ := json.Marshal(policy)
		key = string(data)
	}
	if runtime.GOOS == "linux" {
		for _, path := range []string{"/proc/net/route", "/proc/net/ipv6_route"} {
			if data, err := os.ReadFile(path); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					fields := strings.Fields(line)
					if len(fields) < 10 || fields[0] == "Iface" {
						continue
					}
					// Reference/use counters change with traffic, not routing.
					if path == "/proc/net/route" {
						fields[4], fields[5] = "0", "0"
					} else {
						fields[6], fields[7] = "0", "0"
					}
					key += strings.Join(fields, " ") + "\n"
				}
			}
		}
	}
	// UDP connect asks the kernel for its preferred source address without
	// sending packets or requiring DNS. Also detects address changes with
	// unchanged routes, and provides a fallback on other platforms.
	for _, address := range []string{"1.1.1.1:53", "[2606:4700:4700::1111]:53"} {
		conn, err := net.Dial("udp", address)
		if err != nil {
			key += "|unreachable"
			continue
		}
		key += "|" + conn.LocalAddr().(*net.UDPAddr).IP.String()
		_ = conn.Close()
	}
	return key
}

// NewForTest supplies a deterministic routing snapshot for offline tests.
func NewForTest(base *http.Transport, routeKey func() string, interrupt ...bool) *Transport {
	t := New(base)
	t.routeKey = routeKey
	t.interruptOnChange = len(interrupt) > 0 && interrupt[0]
	return t
}

// NewDefaultTransport applies the saved selection to standard HTTP clients.
func NewDefaultTransport() *Transport { return New(http.DefaultTransport.(*http.Transport)) }

var DefaultClient = &http.Client{Transport: NewDefaultTransport()}
