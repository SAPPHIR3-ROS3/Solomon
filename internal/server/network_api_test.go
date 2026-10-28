package server

import (
	"context"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkAPI(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	request := httptest.NewRequest(http.MethodGet, "/__solomon/network-interfaces", nil)
	response := httptest.NewRecorder()
	handleNetworkInterfaces(response, request)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"number"`) {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	for _, body := range []string{`{"name":"missing"}`, `{"name":"missing","enabled":false}`, `{`} {
		response = httptest.NewRecorder()
		handleNetworkInterfaces(response, httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body)))
		if response.Code != 400 {
			t.Fatalf("accepted %s: %d", body, response.Code)
		}
	}
	response = httptest.NewRecorder()
	handleNetworkInterfaces(response, httptest.NewRequest(http.MethodDelete, "/", nil))
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
}

func TestNetworkRequestSelection(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	rows, err := networkpolicy.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Local {
			continue
		}
		if err := networkpolicy.Set(row.Name, false); err != nil {
			t.Fatal(err)
		}
		for _, address := range row.Addresses {
			ip, _, _ := net.ParseCIDR(address)
			r := httptest.NewRequest("GET", "/", nil)
			r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: ip}))
			w := httptest.NewRecorder()
			if allowNetworkRequest(w, r) || w.Code != 403 {
				t.Fatal("disabled server address allowed")
			}
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}))
	if !allowNetworkRequest(httptest.NewRecorder(), r) {
		t.Fatal("local access blocked")
	}
}

func TestMagicDNSHostAccessDoesNotBlockLocalhost(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	home, err := paths.SolomonHome()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "network.json"), []byte(`{"disabled_magic_dns":{"node.example.ts.net":true,"node":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host    string
		allowed bool
	}{{"node.example.ts.net:64000", false}, {"NODE.EXAMPLE.TS.NET.:64000", false}, {"node:64000", false}, {"localhost:64000", true}, {"127.0.0.1:64000", true}} {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}))
		if got := allowNetworkRequest(httptest.NewRecorder(), r); got != tc.allowed {
			t.Fatalf("%s allowed=%v", tc.host, got)
		}
	}
}
