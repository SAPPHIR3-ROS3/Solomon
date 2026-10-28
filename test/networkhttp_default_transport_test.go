package test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDefaultClientHonorsCustomTransportInstalledAfterInitialization(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("fixture")), Request: r}, nil
	})
	for _, client := range []*http.Client{networkhttp.DefaultClient, {Transport: networkhttp.NewDefaultTransport()}} {
		response, err := client.Get("https://offline.invalid/test")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if string(body) != "fixture" {
			t.Fatal(string(body))
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestCustomDefaultTransportDoesNotBypassSavedSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("fixture")), Request: r}, nil
	})
	client := &http.Client{Transport: networkhttp.NewDefaultTransport()}
	if err := os.WriteFile(filepath.Join(home, "network.json"), []byte(`{"disabled":{"eth0":true},"disabled_magic_dns":{"node.test.ts.net":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"https://node.test.ts.net/", "https://upstream.invalid/"} {
		if response, err := client.Get(url); err == nil {
			response.Body.Close()
			t.Fatalf("custom transport bypassed selection for %s", url)
		}
	}
	if calls != 0 {
		t.Fatal("disallowed request reached custom transport")
	}
	response, err := client.Get("http://localhost:64000/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if calls != 1 {
		t.Fatal("local access blocked")
	}
}
