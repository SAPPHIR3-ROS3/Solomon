package test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
)

func TestMagicDNSSelectionPersistsIndependentlyOfParent(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	rows, err := networkpolicy.ListUnfiltered(networkpolicy.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	var parent networkpolicy.Interface
	var ip net.IP
	for _, row := range rows {
		if !row.Local && len(row.Addresses) > 0 {
			parent = row
			ip, _, _ = net.ParseCIDR(row.Addresses[0])
			break
		}
	}
	if ip == nil {
		t.Skip("no external interface")
	}
	status, _ := json.Marshal(map[string]any{"BackendState": "Running", "CurrentTailnet": map[string]any{"Name": "test", "MagicDNSEnabled": true}, "Self": map[string]any{"DNSName": "node.test.ts.net.", "TailscaleIPs": []string{ip.String()}}})
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "tailscale"), []byte("#!/bin/sh\nprintf '%s' '"+string(status)+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	target := parent.Name + ":magicdns"
	if err := networkpolicy.Set(target, false); err != nil {
		t.Fatal(err)
	}
	p, err := networkpolicy.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"node.test.ts.net", "NODE.TEST.TS.NET.:64000", "node:64000"} {
		if p.AllowedHostname(host) {
			t.Fatalf("allowed disabled alias %s", host)
		}
	}
	if !p.AllowedHostname(ip.String()) || !p.AllowedHostname("localhost:64000") {
		t.Fatal("blocked IP or localhost")
	}
	if !networkpolicy.AllowedLocal(ip) {
		t.Fatal("disabled MagicDNS also disabled parent IP")
	}
	assertState := func(want bool) {
		t.Helper()
		rows, err := networkpolicy.List()
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Name == parent.Name {
				if row.Tailscale == nil || row.Tailscale.MagicDNSEnabled != want {
					t.Fatalf("wrong effective child state: %+v", row)
				}
				return
			}
		}
		t.Fatal("parent missing")
	}
	assertState(false)
	if err := networkpolicy.Set(target, true); err != nil {
		t.Fatal(err)
	}
	assertState(true)
	if err := networkpolicy.Set(parent.Name, false); err != nil {
		t.Fatal(err)
	}
	assertState(false)
	if err := networkpolicy.Set(target, true); err == nil {
		t.Fatal("enabled child under disabled parent")
	}
	if err := networkpolicy.Set(parent.Name, true); err != nil {
		t.Fatal(err)
	}
	assertState(true)
	if err := networkpolicy.Set("missing:magicdns", false); err == nil {
		t.Fatal("accepted unknown parent")
	}
}

func TestDisabledMagicDNSDialDoesNotResolveOrConnect(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	path, err := networkpolicy.PolicyPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := networkpolicy.WritePolicyFile(path, networkpolicy.Policy{DisabledMagicDNS: map[string]bool{"node.test.ts.net": true}}); err != nil {
		t.Fatal(err)
	}
	called := false
	dial := networkpolicy.WrapDial(func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil })
	if _, err := dial(context.Background(), "tcp", "node.test.ts.net:80"); err == nil {
		t.Fatal("disabled alias allowed")
	}
	if called {
		t.Fatal("dialed disabled alias")
	}
}
