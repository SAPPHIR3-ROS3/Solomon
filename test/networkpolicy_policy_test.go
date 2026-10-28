package test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
	"time"
)

func TestSelectionPersistsAndPreservesLocalAccess(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	rows, err := networkpolicy.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Local {
			if networkpolicy.Set(row.Name, false) == nil {
				t.Fatal("disabled loopback")
			}
			if !networkpolicy.AllowedLocal(net.ParseIP("127.0.0.1")) {
				t.Fatal("loopback blocked")
			}
			continue
		}
		if err := networkpolicy.Set(row.Name, false); err != nil {
			t.Fatal(err)
		}
		p, err := networkpolicy.Read()
		if err != nil || !p.Disabled[row.Name] {
			t.Fatalf("selection not persisted: %v %v", p, err)
		}
		for _, address := range row.Addresses {
			ip, _, _ := net.ParseCIDR(address)
			if networkpolicy.AllowedLocal(ip) {
				t.Fatal("disabled address allowed")
			}
		}
		if err := networkpolicy.Set(row.Name, true); err != nil {
			t.Fatal(err)
		}
	}
	if networkpolicy.Set("missing-solomon-interface", false) == nil {
		t.Fatal("accepted unknown interface")
	}
}

func TestMalformedPolicyFailsClosed(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	path, err := networkpolicy.PolicyPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if networkpolicy.AllowedLocal(net.ParseIP("192.0.2.1")) {
		t.Fatal("allowed corrupt policy")
	}
	if _, err := networkpolicy.DialContext(context.Background(), "tcp", "192.0.2.1:80"); err == nil {
		t.Fatal("dial accepted corrupt policy")
	}
}

func TestAllExternalInterfacesDisabled(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	rows, err := networkpolicy.ListUnfiltered(networkpolicy.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if !row.Local {
			if err := networkpolicy.Set(row.Name, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if conn, err := networkpolicy.DialContext(ctx, "tcp", "192.0.2.1:80"); err == nil {
		conn.Close()
		t.Fatal("external dial allowed")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	conn, err := networkpolicy.DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}
