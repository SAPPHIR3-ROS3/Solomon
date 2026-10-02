package test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	guibundle "github.com/SAPPHIR3-ROS3/Solomon/v2026/gui"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestDesktopRuntime_reusesHealthyDaemon(t *testing.T) {
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	// An absent CLI proves that an existing daemon is reused without spawning.
	got, err := serverruntime.EnsureRunning(context.Background(), filepath.Join(t.TempDir(), "missing-solomon"))
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != state.PID || got.URL != state.URL {
		t.Fatalf("different daemon: %#v", got)
	}
}

func TestDesktopRuntime_missingCLIReportsStartupFailure(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	_, err := serverruntime.EnsureRunning(context.Background(), filepath.Join(t.TempDir(), "missing-solomon"))
	if err == nil || !strings.Contains(err.Error(), "did not become ready") {
		t.Fatalf("unexpected startup result: %v", err)
	}
}

func TestDesktopRuntime_productionFrontend(t *testing.T) {
	if !guibundle.Ready() {
		t.Skip("run make gui-build to test the production frontend")
	}
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	for _, path := range []string{"/", "/settings", "/assets/missing.js"} {
		response, err := http.Get(state.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if path == "/assets/missing.js" {
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("missing asset returned %s", response.Status)
			}
		} else if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "id=\"root\"") {
			t.Fatalf("frontend %s: %s %s", path, response.Status, body)
		}
	}
}

func TestDesktopRuntime_nativeOriginPreflight(t *testing.T) {
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	for _, origin := range []string{"wails://wails", "http://wails.localhost", "wails://untrusted", "https://untrusted.example", "null"} {
		request, err := http.NewRequest(http.MethodOptions, state.URL+"/__solomon/projects", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Origin", origin)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if origin == "wails://wails" || origin == "http://wails.localhost" {
			if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != origin {
				t.Fatalf("native preflight rejected: %s %s", origin, response.Status)
			}
		} else if response.StatusCode != http.StatusForbidden {
			t.Fatalf("untrusted origin accepted: %s", origin)
		}
	}
}

func TestDesktopRuntime_proxyAdaptsOriginAndTracksDaemon(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originSeen := r.Header.Get("Origin")
		w.Header().Set("X-Upstream-Origin", originSeen)
		w.Header().Set("Access-Control-Allow-Origin", originSeen)
		w.Write([]byte("daemon-one"))
	}))
	defer daemon.Close()
	if err := serverruntime.SaveState(serverruntime.State{URL: daemon.URL}); err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(serverruntime.DesktopProxy())
	defer proxy.Close()
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/__solomon/projects", nil)
	request.Header.Set("Origin", "wails://wails")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.Header.Get("X-Upstream-Origin") != "http://wails.localhost" || response.Header.Get("Access-Control-Allow-Origin") != "wails://wails" || string(body) != "daemon-one" {
		t.Fatalf("native proxy failed: origin=%s headers=%v body=%s", response.Header.Get("X-Upstream-Origin"), response.Header, body)
	}
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("daemon-two")) }))
	defer second.Close()
	if err := serverruntime.SaveState(serverruntime.State{URL: second.URL}); err != nil {
		t.Fatal(err)
	}
	response, err = http.Get(proxy.URL + "/__solomon/projects")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "daemon-two" {
		t.Fatalf("proxy did not follow daemon restart: %s", body)
	}
	request.Header.Set("Origin", "https://untrusted.example")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("proxy accepted unrelated origin: %s", response.Status)
	}
}

func TestDesktopRuntime_proxyStreamsSSEAndWebSockets(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	gate := make(chan struct{})
	defer func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	}()
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__solomon/events" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: first\n\n"))
			w.(http.Flusher).Flush()
			<-gate
			w.Write([]byte("data: second\n\n"))
			return
		}
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "http://wails.localhost" }}
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		kind, body, err := connection.ReadMessage()
		if err == nil {
			connection.WriteMessage(kind, body)
		}
	}))
	defer daemon.Close()
	if err := serverruntime.SaveState(serverruntime.State{URL: daemon.URL}); err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(serverruntime.DesktopProxy())
	defer proxy.Close()
	response, err := (&http.Client{Timeout: 2 * time.Second}).Get(proxy.URL + "/__solomon/events")
	if err != nil {
		close(gate)
		t.Fatal(err)
	}
	first := make([]byte, len("data: first\n\n"))
	_, err = io.ReadFull(response.Body, first)
	close(gate)
	response.Body.Close()
	if err != nil || string(first) != "data: first\n\n" {
		t.Fatalf("SSE did not flush: %s %v", first, err)
	}
	header := http.Header{"Origin": {"wails://wails"}}
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(proxy.URL, "http")+"/__solomon/terminal", header)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := connection.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	_, body, err := connection.ReadMessage()
	if err != nil || string(body) != "hello" {
		t.Fatalf("WebSocket did not proxy: %s %v", body, err)
	}
}
