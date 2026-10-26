// Package networkhttp keeps HTTP connection pools aligned with system routing.
package networkhttp

import (
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
)

// Transport replaces its connection pool when the system's routes change.
// Existing response bodies remain readable; subsequent requests use a fresh
// connection and let the OS choose its interface. Requests are never replayed.
type Transport struct {
	mu       sync.Mutex
	base     *http.Transport
	current  *http.Transport
	key      string
	routeKey func() string
}

// New clones base, which must not be modified afterward.
func New(base *http.Transport) *Transport {
	return &Transport{base: base.Clone(), routeKey: systemRouteKey}
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
	return current.RoundTrip(req)
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
	key := ""
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
