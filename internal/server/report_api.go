package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

const reportEndpoint = "https://solomon-reports.sapphir3ros3.workers.dev/reports"

var reportDestination = reportEndpoint
var reportClient = &http.Client{Timeout: 20 * time.Second}

func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid report"})
		return
	}
	note := strings.TrimSpace(body.Note)
	if utf8.RuneCountInString(note) > logging.ReportNoteLimit {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "note is too long"})
		return
	}
	logs, err := logging.ReportLogs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to read logs"})
		return
	}
	payload, err := json.Marshal(map[string]string{
		"sentAt":  logging.ReportStamp(),
		"version": reportVersion(),
		"os":      runtime.GOOS + "/" + runtime.GOARCH,
		"note":    logging.RedactReport(note),
		"logs":    logs,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to build report"})
		return
	}
	response, err := reportClient.Post(reportDestination, "application/json", bytes.NewReader(payload))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "unable to send report"})
		return
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "report was rejected"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func reportVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || strings.TrimSpace(info.Main.Version) == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

func SetReportDestinationForTest(destination string) func() {
	previous := reportDestination
	reportDestination = destination
	return func() { reportDestination = previous }
}
