package test

import (
	"encoding/json"
	"testing"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestTailscaleMagicDNSAddress(t *testing.T) {
	const status = `{"BackendState":"Running","CurrentTailnet":{"MagicDNSEnabled":true},"Self":{"DNSName":"server.example.ts.net.","TailscaleIPs":["100.73.30.96","fd7a:115c:a1e0::1"]}}`
	addresses := []serverruntime.ReachableAddress{
		{Kind: "local", Interface: "enp1s0", URL: "http://192.168.1.10:64000"},
		{Kind: "tailscale", Interface: "tailscale0", URL: "http://100.73.30.96:64000"},
	}
	for _, tc := range []struct {
		name      string
		edit      func(map[string]any)
		addresses []serverruntime.ReachableAddress
		want      bool
	}{
		{name: "running with MagicDNS", addresses: addresses, want: true},
		{name: "LAN binding only", addresses: addresses[:1]},
		{name: "loopback binding", addresses: nil},
		{name: "different Tailscale IP", addresses: addresses, edit: func(s map[string]any) { s["Self"].(map[string]any)["TailscaleIPs"] = []string{"100.64.0.2"} }},
		{name: "MagicDNS disabled", addresses: addresses, edit: func(s map[string]any) { s["CurrentTailnet"].(map[string]any)["MagicDNSEnabled"] = false }},
		{name: "logged out", addresses: addresses, edit: func(s map[string]any) { s["BackendState"] = "NeedsLogin" }},
		{name: "missing self", addresses: addresses, edit: func(s map[string]any) { delete(s, "Self") }},
		{name: "missing tailnet", addresses: addresses, edit: func(s map[string]any) { delete(s, "CurrentTailnet") }},
		{name: "empty DNS name", addresses: addresses, edit: func(s map[string]any) { s["Self"].(map[string]any)["DNSName"] = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fixture map[string]any
			if err := json.Unmarshal([]byte(status), &fixture); err != nil {
				t.Fatal(err)
			}
			if tc.edit != nil {
				tc.edit(fixture)
			}
			data, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := serverruntime.TailscaleMagicDNSAddressForTest(tc.addresses, 64000, data)
			if ok != tc.want {
				t.Fatalf("got %+v, ok=%v, want %v", got, ok, tc.want)
			}
			if ok && (got.URL != "http://server.example.ts.net:64000" || got.Kind != "tailscale" || got.Interface != "tailscale0") {
				t.Fatalf("unexpected MagicDNS address: %+v", got)
			}
		})
	}
	if got, ok := serverruntime.TailscaleMagicDNSAddressForTest(addresses, 64000, []byte("invalid JSON")); ok {
		t.Fatalf("accepted invalid status: %+v", got)
	}
}
