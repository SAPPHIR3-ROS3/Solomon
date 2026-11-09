package server

import (
	"bufio"
	"context"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/memoryprofile"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

var requestSequence atomic.Uint64

type diagnosticResponse struct {
	http.ResponseWriter
	status     int
	bytes      int
	failure    string
	writeError error
}

func (w *diagnosticResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *diagnosticResponse) WriteHeader(status int) {
	if w.status == 0 && (status >= 200 || status == http.StatusSwitchingProtocols) {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *diagnosticResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	if err != nil {
		w.writeError = err
	}
	return n, err
}

func (w *diagnosticResponse) Flush() { _ = w.FlushError() }

func (w *diagnosticResponse) FlushError() error {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if err != nil {
		w.writeError = err
	}
	return err
}

func (w *diagnosticResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, buffer, err
}

func requestDiagnostics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := fmt.Sprintf("%x-%x", started.Unix(), requestSequence.Add(1))
		r = r.WithContext(logging.WithRequestID(r.Context(), id))
		response := &diagnosticResponse{ResponseWriter: w}
		w.Header().Set("X-Solomon-Request-ID", id)
		defer func() {
			panicValue := recover()
			status := response.status
			if status == 0 {
				status = http.StatusOK
			}
			params := map[string]any{"operation": requestOperation(r.URL.Path), "request_id": id, "method": r.Method, "route": strings.ReplaceAll(r.Pattern, "/", "."), "status": status, "duration_ms": time.Since(started).Milliseconds(), "response_bytes": response.bytes}
			if response.failure != "" {
				params["error"] = response.failure
			}
			if response.writeError != nil {
				params["write_error"] = response.writeError.Error()
			}
			if r.Context().Err() != nil {
				params["context_error"] = r.Context().Err().Error()
				params["outcome"] = "canceled"
			}
			if panicValue != nil {
				params["panic_type"] = fmt.Sprintf("%T", panicValue)
				params["stack"] = diagnosticStack()
			}
			if status >= 400 || response.failure != "" || response.writeError != nil || r.Context().Err() != nil || panicValue != nil || time.Since(started) >= time.Second || (r.Method != http.MethodGet && r.Method != http.MethodOptions) {
				if status >= 500 || panicValue != nil {
					report := memoryprofile.Default.Report()
					params["peak_observed_rss_bytes"] = report.PeakRSS
					if count := len(report.Samples); count > 0 {
						sample := report.Samples[count-1]
						params["sampled_rss_bytes"] = sample.Server.RSS + sample.Desktop.RSS + sample.WebKit.RSS
						params["go_heap_bytes"] = sample.Heap
					}
				}
				logging.Diagnostic("HTTP request finished", params)
			}
			if panicValue != nil {
				panic(panicValue)
			}
		}()
		next.ServeHTTP(response, r)
	})
}

func recordResponseError(w http.ResponseWriter, err error) {
	if response, ok := w.(*diagnosticResponse); ok && err != nil {
		response.failure = err.Error()
	}
}

func RequestDiagnosticsForTest(next http.Handler) http.Handler { return requestDiagnostics(next) }

func streamFailure(ctx context.Context, stage string, err error) {
	logging.Diagnostic("chat stream write failed", map[string]any{"request_id": logging.RequestID(ctx), "stage": stage, "error": err.Error()})
}

func diagnosticStack() []string {
	pcs := make([]uintptr, 16)
	count := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:count])
	var stack []string
	for {
		frame, more := frames.Next()
		stack = append(stack, fmt.Sprintf("%s:%d", filepath.Base(frame.Function), frame.Line))
		if !more {
			return stack
		}
	}
}

func requestOperation(path string) string {
	known := " projects chats messages file file-operation events commands stop branches history status git-status git-history worktrees research report plan skills terminal models current-model notifications settings gui-settings client-diagnostics health "
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for index, part := range parts {
		if part == "__solomon" {
			parts[index] = "api"
			continue
		}
		if !strings.Contains(known, " "+part+" ") {
			parts[index] = "*"
		}
	}
	return strings.Join(parts, ".")
}
