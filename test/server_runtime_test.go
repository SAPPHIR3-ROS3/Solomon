package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_normalHealth(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	health := getHealthForTest(t, server.URL)
	if !health.OK || health.Server.Mode != "normal" || health.Server.Vite != "stopped" {
		t.Fatalf("unexpected health: %#v", health)
	}
	if health.Server.URL != server.URL || health.Server.LocalURL == "" {
		t.Fatalf("server URLs were not reported: %#v", health.Server)
	}
	if health.API != "not configured" || health.GUI != "not configured" || health.Workers != "not configured" {
		t.Fatalf("unexpected initial service status: %#v", health)
	}
}

func TestServerRuntime_devProxiesFrontendAndStopsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake npm executable is POSIX-only")
	}
	t.Setenv("SOLOMON_TEST_VITE_HELPER", "1")
	t.Setenv("SOLOMON_TEST_BINARY", testBinaryForServer(t))
	t.Setenv("PATH", fakeNPMForServer(t)+string(os.PathListSeparator)+os.Getenv("PATH"))

	frontend := t.TempDir()
	if err := os.Mkdir(filepath.Join(frontend, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(frontend, "package.json"), []byte(`{"scripts":{"dev":"vite"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	server, stop := startServerForTest(t, serverruntime.Options{Mode: "dev", DevDir: frontend})
	health := getHealthForTest(t, server.URL)
	if health.Server.Mode != "dev" || health.Server.Vite != "running" || health.Server.ViteURL == "" {
		t.Fatalf("dev frontend was not reported as running: %#v", health.Server)
	}
	if health.Server.DevDir != frontend {
		t.Fatalf("development directory = %q, want %q", health.Server.DevDir, frontend)
	}

	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "<main>frontend from vite</main>" {
		t.Fatalf("proxy response = %s %q", response.Status, string(body))
	}

	viteURL := health.Server.ViteURL
	stop()
	if _, err := (&http.Client{Timeout: 200 * time.Millisecond}).Get(viteURL); err == nil {
		t.Fatalf("Vite child still responds at %s after server shutdown", viteURL)
	}
}

func startServerForTest(t *testing.T, options serverruntime.Options) (serverruntime.State, func()) {
	t.Helper()
	t.Setenv("SOLOMON_HOME", t.TempDir())
	options.ListenAddr = "127.0.0.1:0"
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() { errs <- serverruntime.Run(ctx, options) }()

	var state serverruntime.State
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		loaded, err := serverruntime.LoadState()
		if err == nil {
			response, requestErr := (&http.Client{Timeout: 100 * time.Millisecond}).Get(loaded.URL + "/health")
			if requestErr == nil && response.StatusCode == http.StatusOK {
				_ = response.Body.Close()
				state = loaded
				break
			}
			if response != nil {
				_ = response.Body.Close()
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state.URL == "" {
		cancel()
		select {
		case err := <-errs:
			t.Fatalf("server did not start: %v", err)
		case <-time.After(time.Second):
			t.Fatal("server did not become healthy")
		}
	}

	stopped := false
	return state, func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-errs:
			if !errors.Is(err, http.ErrServerClosed) {
				t.Fatalf("server shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("server did not stop")
		}
		if _, err := serverruntime.LoadState(); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("server state remains after shutdown: %v", err)
		}
	}
}

func getHealthForTest(t *testing.T, serverURL string) serverruntime.Health {
	t.Helper()
	response, err := http.Get(serverURL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status = %s", response.Status)
	}
	var health serverruntime.Health
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	return health
}

func testBinaryForServer(t *testing.T) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return binary
}

func fakeNPMForServer(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\nexec \"$SOLOMON_TEST_BINARY\" -test.run=TestServerViteHelperProcess -- vite \"$@\"\n"
	path := filepath.Join(directory, "npm")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestServerViteHelperProcess(t *testing.T) {
	if os.Getenv("SOLOMON_TEST_VITE_HELPER") != "1" {
		return
	}
	port := ""
	for index, arg := range os.Args {
		if arg == "--port" && index+1 < len(os.Args) {
			port = os.Args[index+1]
			break
		}
	}
	if port == "" {
		fmt.Fprintln(os.Stderr, "fake Vite: missing --port")
		os.Exit(2)
	}
	server := &http.Server{
		Addr: "127.0.0.1:" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, "<main>frontend from vite</main>")
		}),
	}
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-interrupts
		_ = server.Close()
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
