package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestProjectPDFPreferences(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	base := server.URL + "/__solomon/projects"
	createProject := func() string {
		t.Helper()
		response := postJSONForServerTest(t, base, map[string]string{"path": t.TempDir()})
		var result struct{ Project struct{ ID string } }
		decodeServerTestJSON(t, response, &result)
		if result.Project.ID == "" {
			t.Fatal("missing project ID")
		}
		return base + "/" + result.Project.ID + "/pdf-preferences"
	}
	first, second := createProject(), createProject()

	put := func(body string, status int) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPut, first, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("PDF preference save returned %s, want %d", response.Status, status)
		}
	}
	read := func(route string) *database.PDFPreferences {
		t.Helper()
		response, err := http.Get(route)
		if err != nil {
			t.Fatal(err)
		}
		var prefs *database.PDFPreferences
		decodeServerTestJSON(t, response, &prefs)
		return prefs
	}

	put(`{"continuous":true,"direction":"horizontal","twoPages":true,"zoom":1.25,"fitMode":"width"}`, http.StatusNoContent)
	put(`{"direction":"vertical","zoom":4}`, http.StatusBadRequest)
	if got := read(first); got == nil || !got.Continuous || !got.TwoPages || got.Direction != "horizontal" ||
		got.Zoom != 1.25 || got.FitMode == nil || *got.FitMode != "width" {
		t.Fatalf("saved preferences = %+v", got)
	}
	if got := read(second); got != nil {
		t.Fatalf("second project inherited preferences: %+v", got)
	}
	put(`{"continuous":false,"direction":"horizontal","twoPages":false,"zoom":1.5,"fitMode":null}`, http.StatusNoContent)
	if got := read(first); got == nil || got.Continuous || got.TwoPages || got.Zoom != 1.5 || got.FitMode != nil {
		t.Fatalf("updated preferences = %+v", got)
	}
}
