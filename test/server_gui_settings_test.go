package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_GUISettingsArePersistedAndDoNotExposeSecrets(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	path, err := paths.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[providers.OpenAI]\napi_key='fixture-secret'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	url := server.URL + "/__solomon/gui-settings"
	for _, origin := range []string{"http://wails.localhost", "https://wails.localhost", "wails://wails"} {
		request, err := http.NewRequest(http.MethodOptions, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Origin", origin)
		request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
		request.Header.Set("Access-Control-Request-Headers", "content-type")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != origin || !strings.Contains(response.Header.Get("Access-Control-Allow-Methods"), "PATCH") {
			t.Fatalf("desktop PATCH preflight failed for %s: %d, %v", origin, response.StatusCode, response.Header)
		}
	}
	patch := func(body string) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPatch, url, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	response := patch(`{"chat":{"autoCloseToolCalls":true,"startToolCallsCollapsed":false}}`)
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("patch status=%d: %s", response.StatusCode, data)
	}
	if bytes.Contains(data, []byte("fixture-secret")) || bytes.Contains(data, []byte("providers")) {
		t.Fatal("GUI API exposed provider config")
	}
	var gui config.GUISettings
	if err := json.Unmarshal(data, &gui); err != nil {
		t.Fatal(err)
	}
	if gui.Chat.AutoCloseToolCalls == nil || !*gui.Chat.AutoCloseToolCalls {
		t.Fatal("missing persisted preference")
	}
	persisted, err := config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Chat.AutoCloseToolCalls == nil || !*persisted.Chat.AutoCloseToolCalls {
		t.Fatal("API did not write config")
	}
	response, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	decodeServerTestJSON(t, response, &gui)
	if gui.Chat.StartToolCallsCollapsed == nil || *gui.Chat.StartToolCallsCollapsed {
		t.Fatal("false preference did not round trip")
	}
	for _, body := range []string{`{"chat":{}}`, `{"chat":{"autoCloseToolCalls":"true"}}`, `{"chat":{"autoCloseToolCalls":null}}`} {
		response := patch(body)
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid patch accepted: %s (%d)", body, response.StatusCode)
		}
	}
	response, err = http.Post(url, "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatal("unsupported method accepted")
	}
}
