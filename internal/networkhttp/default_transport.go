package networkhttp

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
)

// NewDefaultTransport follows the standard client's current default transport,
// including custom RoundTrippers used by integrations and offline test fixtures.
func NewDefaultTransport() http.RoundTripper { return &defaultTransport{} }

type defaultTransport struct {
	mu      sync.Mutex
	base    *http.Transport
	current *Transport
}

func (t *defaultTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	base := http.DefaultTransport
	if standard, ok := base.(*http.Transport); ok {
		t.mu.Lock()
		if t.current == nil || t.base != standard {
			if t.current != nil {
				t.current.CloseIdleConnections()
			}
			t.base = standard
			t.current = New(standard)
		}
		current := t.current
		t.mu.Unlock()
		return current.RoundTrip(request)
	}
	policy, err := networkpolicy.Read()
	if err != nil {
		return nil, err
	}
	if !policy.AllowedHostname(request.URL.Hostname()) {
		return nil, errors.New("Solomon access via this MagicDNS name is disabled")
	}
	// A custom RoundTripper owns its dial behavior. Do not silently bypass an
	// interface restriction that cannot be installed on that transport.
	ip := net.ParseIP(request.URL.Hostname())
	local := (ip != nil && ip.IsLoopback()) || strings.EqualFold(strings.TrimSuffix(request.URL.Hostname(), "."), "localhost")
	if len(policy.Disabled) > 0 && !local {
		return nil, errors.New("custom HTTP transport cannot enforce network interface selection")
	}
	return base.RoundTrip(request)
}

func (t *defaultTransport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current != nil {
		t.current.CloseIdleConnections()
	}
}

var DefaultClient = &http.Client{Transport: NewDefaultTransport()}
