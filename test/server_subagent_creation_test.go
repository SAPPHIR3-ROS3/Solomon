package test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_CreateSubagent(t *testing.T) {
	withMockRolesModelLister(t)
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	configPath, err := paths.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	fixture := "user_name = 'Fixture'\n[providers.p]\napi_key = 'fixture-secret'\n[roles.table]\ncharacteristics = ['reasoning']\n"
	if err := os.WriteFile(configPath, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	post := func(body string, status int) []byte {
		t.Helper()
		response, err := http.Post(server.URL+"/__solomon/subagents", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != status {
			t.Fatalf("status=%d want=%d: %s", response.StatusCode, status, data)
		}
		return data
	}
	invalid := []string{
		`{"provider":"","model":"m","scores":[{"id":"reasoning","value":80}]}`,
		`{"provider":"missing","model":"m","scores":[{"id":"reasoning","value":80}]}`,
		`{"provider":"p","model":"unknown","scores":[{"id":"reasoning","value":80}]}`,
		`{"provider":"p","model":"m","scores":[]}`,
		`{"provider":"p","model":"m","scores":[{"id":"reasoning","value":101}]}`,
		`{"provider":"p","model":"m","scores":[{"id":"reasoning","value":80},{"id":"unknown","value":50}]}`,
		`{"provider":"p","model":"m","scores":[{"id":"reasoning","value":80},{"id":"reasoning","value":50}]}`,
	}
	for _, body := range invalid {
		post(body, http.StatusBadRequest)
	}
	before, _ := os.ReadFile(configPath)
	if string(before) != fixture {
		t.Fatal("invalid requests modified config")
	}
	body := `{"provider":" p ","model":" m ","detail":" Explorer ","scores":[{"id":"reasoning","value":80}]}`
	data := post(body, http.StatusOK)
	if bytes.Contains(data, []byte("fixture-secret")) || !bytes.Contains(data, []byte(`"title":"m"`)) {
		t.Fatalf("unexpected response: %s", data)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roles.Subagent) != 1 {
		t.Fatalf("roles=%+v", cfg.Roles.Subagent)
	}
	entry := cfg.Roles.Subagent[0]
	if entry.Provider != "p" || entry.Model != "m" || entry.Description != "Explorer" || entry.Scores["reasoning"] != 80 {
		t.Fatalf("entry=%+v", entry)
	}
	saved, _ := os.ReadFile(configPath)
	post(body, http.StatusBadRequest)
	after, _ := os.ReadFile(configPath)
	if !bytes.Equal(saved, after) {
		t.Fatal("duplicate modified config")
	}
	if err := os.WriteFile(configPath, []byte("[providers.p]\napi_key='fixture-secret'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	post(body, http.StatusBadRequest)
}
