package server

import (
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

var clientDiagnosticSource = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,160}$`)
var clientDiagnosticWindow struct {
	sync.Mutex
	started time.Time
	count   int
}

func handleClientDiagnostic(w http.ResponseWriter, r *http.Request) {
	var event struct {
		Stage  string `json:"stage"`
		Name   string `json:"name"`
		Source string `json:"source"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if err := decodeJSONBody(w, r, &event, 1024); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch event.Stage {
	case "runtime", "promise", "react-uncaught", "react-caught", "react-recoverable", "startup-identities", "background-refresh", "settings-save", "autocomplete":
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch event.Name {
	case "Error", "TypeError", "RangeError", "SyntaxError", "ReferenceError", "URIError", "EvalError", "AggregateError", "UnknownError":
	default:
		event.Name = "UnknownError"
	}
	if event.Source != "" && !clientDiagnosticSource.MatchString(event.Source) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	clientDiagnosticWindow.Lock()
	if time.Since(clientDiagnosticWindow.started) >= time.Minute {
		clientDiagnosticWindow.started = time.Now()
		clientDiagnosticWindow.count = 0
	}
	allowed := clientDiagnosticWindow.count < 20
	if allowed {
		clientDiagnosticWindow.count++
	}
	clientDiagnosticWindow.Unlock()
	if allowed {
		logging.Diagnostic("GUI error observed", map[string]any{"request_id": logging.RequestID(r.Context()), "stage": event.Stage, "error_name": event.Name, "source": event.Source, "line": event.Line, "column": event.Column})
	}
	w.WriteHeader(http.StatusAccepted)
}

func ClientDiagnosticsForTest() http.Handler { return http.HandlerFunc(handleClientDiagnostic) }
