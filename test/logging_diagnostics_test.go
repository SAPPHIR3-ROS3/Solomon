package test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func diagnosticLogs(t *testing.T) func() string {
	t.Helper()
	logging.LogInit(logging.INFO_LOG_LEVEL)
	dir := t.TempDir()
	if err := logging.Configure(logging.Config{Dir: dir, WriteFile: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logging.Configure(logging.Config{WriteConsole: true}) })
	return func() string {
		entries, err := filepath.Glob(filepath.Join(dir, "*.log"))
		if err != nil {
			t.Fatal(err)
		}
		var text strings.Builder
		for _, entry := range entries {
			data, err := os.ReadFile(entry)
			if err != nil {
				t.Fatal(err)
			}
			text.Write(data)
		}
		return text.String()
	}
}

func TestDiagnosticProcessChild(t *testing.T) {
	if os.Getenv("SOLOMON_DIAGNOSTIC_CHILD") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func TestDiagnosticProcessDistinguishesDeadlineFromCancellation(t *testing.T) {
	read := diagnosticLogs(t)
	for _, deadline := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if deadline {
			cancel()
			ctx, cancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
		}
		ctx = logging.WithRequestID(ctx, "request-123")
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDiagnosticProcessChild$", "secret-argument")
		cmd.Env = append(os.Environ(), "SOLOMON_DIAGNOSTIC_CHILD=1", "PRIVATE_TOKEN=secret-environment")
		started := time.Now()
		if err := cmd.Start(); err != nil {
			cancel()
			t.Fatal(err)
		}
		if !deadline {
			cancel()
		}
		err := cmd.Wait()
		logging.ProcessFinished(ctx, cmd, "test.process", started, err)
		cancel()
	}
	text := read()
	for _, want := range []string{"outcome:canceled", "outcome:timeout", "request_id:request-123", "pid:", "process_state:", "context_error:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if _, err := os.Stat("/proc/vmstat"); err == nil && !strings.Contains(text, "system_oom_kills:") {
		t.Fatalf("signal exit omitted Linux OOM evidence: %s", text)
	}
	for _, secret := range []string{"secret-argument", "secret-environment", "PRIVATE_TOKEN"} {
		if strings.Contains(text, secret) {
			t.Fatalf("process log leaked %q", secret)
		}
	}
}

func TestRequestDiagnosticsCorrelatesFailuresWithoutLoggingQueryOrBody(t *testing.T) {
	read := diagnosticLogs(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /failure", func(w http.ResponseWriter, r *http.Request) {
		if logging.RequestID(r.Context()) == "" {
			t.Error("request ID missing from context")
		}
		http.Error(w, "private response content", http.StatusInternalServerError)
	})
	request := httptest.NewRequest(http.MethodPost, "/failure?token=secret-query", strings.NewReader("secret-body"))
	response := httptest.NewRecorder()
	server.RequestDiagnosticsForTest(mux).ServeHTTP(response, request)
	id := response.Header().Get("X-Solomon-Request-ID")
	text := read()
	if id == "" || !strings.Contains(text, "request_id:"+id) || !strings.Contains(text, "status:500") || !strings.Contains(text, "route:POST .failure") {
		t.Fatalf("missing request diagnostics: id=%s log=%s", id, text)
	}
	for _, secret := range []string{"secret-query", "secret-body", "private response content"} {
		if strings.Contains(text, secret) {
			t.Fatalf("request log leaked %q", secret)
		}
	}
}

func TestRequestDiagnosticsPreservesStreamingAndSuppressesHealthyPolling(t *testing.T) {
	read := diagnosticLogs(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /poll", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") })
	mux.HandleFunc("GET /stream", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "data: hello\n\n")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
	})
	handler := server.RequestDiagnosticsForTest(mux)
	poll := httptest.NewRecorder()
	handler.ServeHTTP(poll, httptest.NewRequest(http.MethodGet, "/poll", nil))
	stream := httptest.NewRecorder()
	handler.ServeHTTP(stream, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if poll.Body.String() != "ok" || !stream.Flushed || stream.Body.String() != "data: hello\n\n" {
		t.Fatal("middleware changed response or flushing")
	}
	if text := read(); text != "" {
		t.Fatalf("healthy reads should not generate logs: %s", text)
	}
}

func TestDiagnosticRedactsSecretsAndPreventsMultilineEntries(t *testing.T) {
	read := diagnosticLogs(t)
	logging.Diagnostic("diagnostic failure", map[string]any{"error": "Bearer private-token\nforged log entry", "large": strings.Repeat("x", 10000)})
	text := read()
	if strings.Contains(text, "private-token") || strings.Count(text, "\n") != 1 || len(text) > 4500 {
		t.Fatalf("unsafe diagnostic record: %q", text)
	}
}

func TestOutboundDiagnosticsOmitsCredentialsAndURL(t *testing.T) {
	read := diagnosticLogs(t)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer remote.Close()
	transport := networkhttp.New(http.DefaultTransport.(*http.Transport))
	defer transport.CloseIdleConnections()
	request, err := http.NewRequest(http.MethodGet, remote.URL+"/secret-path?token=secret-query", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer private-token")
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	text := read()
	if !strings.Contains(text, "outbound HTTP request finished") || !strings.Contains(text, "status:502") {
		t.Fatalf("missing outbound failure: %s", text)
	}
	for _, secret := range []string{"private-token", "secret-path", "secret-query"} {
		if strings.Contains(text, secret) {
			t.Fatalf("outbound log leaked %q", secret)
		}
	}
}

func TestRequestDiagnosticsRethrowsPanics(t *testing.T) {
	read := diagnosticLogs(t)
	handler := server.RequestDiagnosticsForTest(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(errors.New("private panic payload")) }))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic was swallowed")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))
	}()
	text := read()
	if !strings.Contains(text, "panic_type:") || strings.Contains(text, "private panic payload") {
		t.Fatalf("missing or unsafe panic diagnostics: %s", text)
	}
}

func TestDiagnosticFailuresRespectWarningLevelWithoutCreatingNotifications(t *testing.T) {
	read := diagnosticLogs(t)
	before, err := logging.ReadNotifications()
	if err != nil {
		t.Fatal(err)
	}
	if err := logging.SetGlobalLevel(logging.WARNING_LOG_LEVEL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logging.SetGlobalLevel(logging.INFO_LOG_LEVEL) })
	logging.Diagnostic("ordinary diagnostic", nil)
	logging.Diagnostic("failed diagnostic", map[string]any{"error": "token=private-value password=another-value"})
	text := read()
	if strings.Contains(text, "ordinary diagnostic") || !strings.Contains(text, "failed diagnostic") || strings.Contains(text, "private-value") || strings.Contains(text, "another-value") {
		t.Fatalf("incorrect severity or secret handling: %s", text)
	}
	after, err := logging.ReadNotifications()
	if err != nil || len(after) != len(before) {
		t.Fatalf("diagnostics created UI notifications: before=%d after=%d error=%v", len(before), len(after), err)
	}
}

func TestClientDiagnosticsAcceptsMetadataAndRejectsPaths(t *testing.T) {
	read := diagnosticLogs(t)
	handler := server.ClientDiagnosticsForTest()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"stage":"runtime","name":"TypeError","source":"index-abc.js","line":10,"column":20,"message":"private-content"}`)))
	if response.Code != http.StatusAccepted {
		t.Fatalf("valid diagnostic rejected: %d", response.Code)
	}
	text := read()
	if !strings.Contains(text, "error_name:TypeError") || !strings.Contains(text, "line:10") || strings.Contains(text, "private-content") {
		t.Fatalf("missing metadata or leaked content: %s", text)
	}
	for _, body := range []string{
		`{"stage":"runtime","source":"/private/file.js?token=value"}`,
		`{"stage":"arbitrary-secret","name":"TypeError"}`,
		strings.Repeat("x", 2048),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid diagnostic accepted: %d", response.Code)
		}
	}
}
