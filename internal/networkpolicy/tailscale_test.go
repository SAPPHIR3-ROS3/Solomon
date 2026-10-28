package networkpolicy

import (
	"encoding/json"
	"testing"
)

func TestTailscaleMetadataUsesOwnDeviceAccount(t *testing.T) {
	data := []byte(`{"BackendState":"Running","CurrentTailnet":{"Name":"team.example","MagicDNSSuffix":"example.ts.net"},"Self":{"UserID":9007199254740993,"TailscaleIPs":["100.64.0.2"]},"User":{"1":{"LoginName":"wrong@example.com"},"9007199254740993":{"LoginName":"right@example.com"}}}`)
	rows := []Interface{{Name: "custom-tun", Type: "tunnel", Addresses: []string{"100.64.0.2/32"}}, {Name: "eth0", Addresses: []string{"192.168.1.2/24"}}}
	applyTailscaleStatus(rows, data)
	if rows[0].Tailscale == nil || rows[0].Tailscale.Account != "right@example.com" || rows[0].Tailscale.Tailnet != "team.example" || rows[0].DisplayName() != "Tailscale" {
		t.Fatalf("wrong metadata: %+v", rows[0])
	}
	if rows[1].Tailscale != nil {
		t.Fatal("attached tailnet to unrelated interface")
	}
}
func TestTailscaleMissingStatusAndTaggedDevice(t *testing.T) {
	for _, data := range []string{`{`, `{}`, `{"BackendState":"NeedsLogin","CurrentTailnet":{"Name":"old"},"Self":{"TailscaleIPs":["100.64.0.2"]}}`} {
		rows := []Interface{{Name: "tun", Addresses: []string{"100.64.0.2/32"}}}
		applyTailscaleStatus(rows, []byte(data))
		if rows[0].Tailscale != nil {
			t.Fatal("displayed stale metadata")
		}
	}
	rows := []Interface{{Name: "tun", Addresses: []string{"100.64.0.2/32"}}}
	applyTailscaleStatus(rows, []byte(`{"BackendState":"Running","CurrentTailnet":{"Name":"team"},"Self":{"UserID":1,"Tags":["tag:server"],"TailscaleIPs":["100.64.0.2"]},"User":{"1":{"LoginName":"tagged-devices"}}}`))
	if rows[0].Tailscale == nil || !rows[0].Tailscale.Tagged || rows[0].Tailscale.Account != "" {
		t.Fatal("tagged device presented as user")
	}
}

func TestMagicDNSOnlyWhenAvailable(t *testing.T) {
	for _, tc := range []struct {
		name, state, dns string
		enabled          bool
		want             string
	}{
		{"enabled", "Running", "server.example.ts.net.", true, "server.example.ts.net"},
		{"disabled", "Running", "server.example.ts.net.", false, ""},
		{"stopped", "Stopped", "server.example.ts.net.", true, ""},
		{"missing name", "Running", "", true, ""},
		{"invalid name", "Running", "bad name.example.ts.net", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"BackendState": tc.state, "CurrentTailnet": map[string]any{"Name": "team", "MagicDNSEnabled": tc.enabled}, "Self": map[string]any{"DNSName": tc.dns, "TailscaleIPs": []string{"100.64.0.2"}}})
			rows := []Interface{{Name: "tun", Addresses: []string{"100.64.0.2/32"}}}
			applyTailscaleStatus(rows, data)
			if rows[0].Tailscale == nil || rows[0].Tailscale.MagicDNS != tc.want {
				t.Fatalf("unexpected MagicDNS: %+v", rows[0].Tailscale)
			}
		})
	}
}
