package test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/memoryprofile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_memoryDiagnostics(t *testing.T) {
	state, stop := startServerForTest(t, server.Options{})
	defer stop()
	response, err := http.Get(state.URL + "/__solomon/memory")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("diagnostics response: %d", response.StatusCode)
	}
	var report memoryprofile.Report
	if err := json.NewDecoder(response.Body).Decode(&report); err != nil {
		t.Fatal(err)
	}
	if report.Scope != "solomon-owned-processes" || len(report.Samples) > memoryprofile.SampleLimit {
		t.Fatalf("diagnostics: %+v", report)
	}
	encoded, _ := json.Marshal(report)
	for _, field := range []string{"pid", "executable", "identity", "path", "content"} {
		if strings.Contains(string(encoded), field) {
			t.Fatalf("private field: %s", field)
		}
	}
}
