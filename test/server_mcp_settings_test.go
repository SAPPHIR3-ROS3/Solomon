package test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_McpTogglePersistsAndPreservesConfig(t *testing.T) {
	state, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	path := filepath.Join(t.TempDir(), "mcp.json")
	t.Setenv("SOLOMON_MCP_CONFIG", path)
	original := `{"extension":{"keep":true},"mcpServers":{"one":{"command":"test","env":{"TOKEN":"$UNEXPANDED_SECRET"},"custom":42},"two":{"command":"test"}}}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, disabled := range []bool{true, false} {
		body, _ := json.Marshal(map[string]any{"id": "one", "disabled": disabled})
		response, err := http.Post(state.URL+"/__solomon/mcps", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("toggle: %d %s %v", response.StatusCode, data, err)
		}
		if strings.Contains(string(data), "UNEXPANDED_SECRET") {
			t.Fatal("catalog exposes secret")
		}
		var payload struct {
			MCPs []struct {
				ID       string
				Disabled bool
			} `json:"mcps"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.MCPs) != 2 || payload.MCPs[0].ID != "one" || payload.MCPs[0].Disabled != disabled {
			t.Fatalf("catalog: %+v", payload)
		}
		persisted, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var actual, expected map[string]any
		json.Unmarshal(persisted, &actual)
		json.Unmarshal([]byte(original), &expected)
		expected["mcpServers"].(map[string]any)["one"].(map[string]any)["disabled"] = disabled
		a, _ := json.Marshal(actual)
		e, _ := json.Marshal(expected)
		if string(a) != string(e) {
			t.Fatalf("configuration changed: %s", a)
		}
		if disabled {
			cfg, err := mcp.LoadConfig()
			if err != nil || len(cfg.Servers) != 1 || cfg.Servers[0].Name != "two" {
				t.Fatalf("disabled server loaded: %+v %v", cfg, err)
			}
		}
	}
	before, _ := os.ReadFile(path)
	for _, body := range []string{`{}`, `{"id":"one"}`, `{"id":"one","disabled":null}`, `{"id":"missing","disabled":true}`, `{"id":"one","disabled":"true"}`} {
		response, err := http.Post(state.URL+"/__solomon/mcps", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid update %s: %d", body, response.StatusCode)
		}
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid update changed config")
	}
}
