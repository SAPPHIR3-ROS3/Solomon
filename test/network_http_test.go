package test

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteChangeReplacesConnectionPool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.RemoteAddr)
	}))
	defer server.Close()
	route := "wifi"
	transport := networkhttp.NewForTest(http.DefaultTransport.(*http.Transport), func() string { return route })
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	request := func() string {
		t.Helper()
		response, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	wifi := request()
	if next := request(); next != wifi {
		t.Fatal("unchanged route did not reuse connection")
	}
	route = "ethernet"
	ethernet := request()
	if ethernet == wifi {
		t.Fatal("new route reused old connection")
	}
	if next := request(); next != ethernet {
		t.Fatal("new route did not reuse its own connection")
	}
	transport.PrepareRetry()
	if next := request(); next == ethernet {
		t.Fatal("retry reused pool despite unchanged route")
	}
	route = "wifi"
	if next := request(); next == ethernet || next == wifi {
		t.Fatal("returning route reused a retired pool")
	}
}

func TestRouteChangePreservesActiveResponse(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stream" {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-release
		}
		_, _ = io.WriteString(w, "complete")
	}))
	defer server.Close()
	defer close(release)
	route := "wifi"
	transport := networkhttp.NewForTest(http.DefaultTransport.(*http.Transport), func() string { return route })
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	response, err := client.Get(server.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	route = "ethernet"
	next, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = next.Body.Close()
	// Unblock the original handler before reading its body.
	release <- struct{}{}
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "complete" {
		t.Fatalf("active response interrupted: %q, %v", body, err)
	}
}
