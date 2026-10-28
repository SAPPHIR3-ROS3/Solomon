package commands

import (
	"bytes"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
	"strings"
	"testing"
)

func TestInterfaceCommand(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	var out bytes.Buffer
	deps := Deps{Out: &out}
	if matched, err := DispatchBuiltinSlash(deps, []string{"/interface"}, "interface"); !matched || err != nil {
		t.Fatalf("%v %v", matched, err)
	}
	if !strings.Contains(out.String(), "/interface <n> <on|off>") {
		t.Fatal(out.String())
	}
	for _, parts := range [][]string{{"/interface", "0", "off"}, {"/interface", "1"}, {"/interface", "1", "invalid"}, {"/interface", "x", "on"}} {
		if Interface(deps, parts) == nil {
			t.Fatalf("accepted %v", parts)
		}
	}
	rows, err := networkpolicy.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Local {
			continue
		}
		if err := Interface(deps, []string{"/interface", fmt.Sprint(row.Number), "off"}); err != nil {
			t.Fatal(err)
		}
		p, err := networkpolicy.Read()
		if err != nil || !p.Disabled[row.Name] {
			t.Fatalf("%v %v", p, err)
		}
		break
	}
}

func TestInterfaceListIsOnePlainListWithNestedMagicDNS(t *testing.T) {
	output := formatInterfaceList([]networkpolicy.Interface{
		{Number: 1, Name: "lo", Type: "loopback", Enabled: true, Up: true, Addresses: []string{"127.0.0.1/8", "::1/128"}},
		{Number: 4, Name: "tailscale0", Type: "tailscale", Enabled: true, Up: true, Addresses: []string{"100.64.0.2/32"}, Tailscale: &networkpolicy.TailscaleInfo{Account: "user@example.com", Tailnet: "team", MagicDNS: "node.team.ts.net", MagicDNSEnabled: true}},
	})
	if strings.Contains(output, "SYSTEM") || strings.Contains(output, "\x1b[") {
		t.Fatalf("system notification wrapper: %q", output)
	}
	if strings.Count(output, "Network interfaces") != 1 {
		t.Fatal(output)
	}
	want := "1. Localhost (lo): on (link up) 127.0.0.1/8\n4. Tailscale (tailscale0): on (link up) 100.64.0.2/32\n   Account: user@example.com | Tailnet: team\n   4.1 MagicDNS: on node.team.ts.net\n"
	if !strings.Contains(output, want) {
		t.Fatalf("list is not cohesive or correctly nested: %s", output)
	}
	if strings.Contains(output, "::1") {
		t.Fatal("IPv6 display returned")
	}
}
