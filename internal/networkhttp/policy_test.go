package networkhttp

import (
	"context"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPolicyChangeReplacesPreviouslyAllowedPool(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer server.Close()
	dialer := net.Dialer{}
	base := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}}
	transport := New(base)
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	response, err := client.Get("http://192.0.2.1/")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	rows, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows {
		if row.Flags&net.FlagLoopback == 0 {
			count++
			if err := networkpolicy.Set(row.Name, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	if count == 0 {
		t.Skip("no external interfaces")
	}
	if response, err := client.Get("http://192.0.2.1/"); err == nil {
		response.Body.Close()
		t.Fatal("reused a connection after disabling its interfaces")
	}
}
