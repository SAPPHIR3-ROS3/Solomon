package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
)

func TestLinuxInterfaceTypeUsesMetadataInsteadOfNames(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"renamed-wifi", map[string]string{"type": "1\n", "uevent": "DEVTYPE=wlan\n", "device": ""}, "wifi"},
		{"renamed-ethernet", map[string]string{"type": "1\n", "device": ""}, "ethernet"},
		{"phy-wifi", map[string]string{"type": "1\n", "phy80211": "", "device": ""}, "wifi"},
		{"renamed-bridge", map[string]string{"type": "1\n", "bridge": ""}, "bridge"},
		{"renamed-vpn", map[string]string{"type": "65534\n", "uevent": "DEVTYPE=wireguard\n"}, "wireguard"},
		{"renamed-tun", map[string]string{"type": "65534\n", "tun_flags": "0x5001\n"}, "tunnel"},
		{"eth0", map[string]string{"type": "1\n"}, "unknown"},
		{"wlan0", map[string]string{}, "unknown"},
		{"lo-renamed", map[string]string{"type": "772\n"}, "loopback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, tc.name)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for name, value := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if got := networkpolicy.LinuxInterfaceType(root, tc.name); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
func TestInterfaceNameFallback(t *testing.T) {
	for _, kind := range []string{"", "unknown", "future-type"} {
		i := networkpolicy.Interface{Name: "custom0", Type: kind}
		if i.DisplayName() != "custom0" {
			t.Fatal("lost technical name")
		}
	}
	if got := networkpolicy.InterfaceType("irrelevant", true); got != "loopback" {
		t.Fatal(got)
	}
}
