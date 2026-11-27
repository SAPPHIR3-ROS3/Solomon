package test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestDeviceLinksAccess(t *testing.T) {
	setSolomonHome(t, t.TempDir())
	cases := []struct {
		name, host, remote, origin string
		want                       int
	}{
		{"local", "localhost:8080", "127.0.0.1:1234", "", http.StatusOK},
		{"remote client", "localhost:8080", "192.0.2.5:1234", "", http.StatusForbidden},
		{"untrusted origin", "localhost:8080", "127.0.0.1:1234", "https://example.test", http.StatusForbidden},
		{"untrusted host", "example.test:8080", "127.0.0.1:1234", "", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/__solomon/device-links", nil)
			request.RemoteAddr = tc.remote
			request.Header.Set("Origin", tc.origin)
			response := httptest.NewRecorder()
			server.HandleDeviceLinks(response, request)
			if response.Code != tc.want {
				t.Fatalf("status %d, want %d", response.Code, tc.want)
			}
			if tc.want == http.StatusOK {
				var links []networkpolicy.Link
				if err := json.Unmarshal(response.Body.Bytes(), &links); err != nil || links == nil {
					t.Fatalf("expected a JSON list of links, got %q", response.Body.String())
				}
			}
		})
	}
}

func TestDeviceAccessDefaultsToDenied(t *testing.T) {
	setSolomonHome(t, t.TempDir())
	rows, err := networkpolicy.ListUnfiltered(networkpolicy.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	source := net.ParseIP("192.0.2.10")
	for _, row := range rows {
		if row.Local || len(row.Addresses) == 0 {
			continue
		}
		destination, _, err := net.ParseCIDR(row.Addresses[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, allowed := range []bool{false, true, false} {
			if err := networkpolicy.SetDevice(row.Name, source.String(), allowed); err != nil {
				t.Fatal(err)
			}
			policy, err := networkpolicy.Read()
			if err != nil {
				t.Fatal(err)
			}
			if policy.AllowsDevice(destination, source) != allowed {
				t.Fatalf("expected device access %v", allowed)
			}
			if policy.AllowsDevice(destination, net.ParseIP("192.0.2.11")) {
				t.Fatal("unlisted device was allowed")
			}
		}
		return
	}
	t.Skip("no non-loopback interface available")
}
