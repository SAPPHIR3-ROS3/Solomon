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
	for _, enabled := range []bool{true, false} {
		body, err := json.Marshal(map[string]any{"chat": map[string]bool{"startReasoningCollapsed": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		response = patch(string(body))
		decodeServerTestJSON(t, response, &gui)
		if gui.Chat.StartReasoningCollapsed == nil || *gui.Chat.StartReasoningCollapsed != enabled {
			t.Fatal("reasoning preference did not round trip")
		}
		persisted, err := config.ReadGUISettings()
		if err != nil {
			t.Fatal(err)
		}
		if persisted.Chat.StartReasoningCollapsed == nil || *persisted.Chat.StartReasoningCollapsed != enabled {
			t.Fatal("reasoning preference was not persisted")
		}
		if persisted.Chat.AutoCloseToolCalls == nil || !*persisted.Chat.AutoCloseToolCalls || persisted.Chat.StartToolCallsCollapsed == nil || *persisted.Chat.StartToolCallsCollapsed {
			t.Fatal("reasoning update changed other chat preferences")
		}
	}
	for _, enabled := range []bool{true, false} {
		body, err := json.Marshal(map[string]any{"chat": map[string]bool{"floatingLetters": enabled}})
		if err != nil {
			t.Fatal(err)
		}
		response = patch(string(body))
		decodeServerTestJSON(t, response, &gui)
		persisted, err := config.ReadGUISettings()
		if err != nil {
			t.Fatal(err)
		}
		if gui.Chat.FloatingLetters == nil || *gui.Chat.FloatingLetters != enabled || persisted.Chat.FloatingLetters == nil || *persisted.Chat.FloatingLetters != enabled {
			t.Fatal("floating letters preference did not persist and round trip")
		}
		if persisted.Chat.AutoCloseToolCalls == nil || !*persisted.Chat.AutoCloseToolCalls {
			t.Fatal("floating letters update changed other preferences")
		}
	}
	for _, density := range []string{"low", "medium", "high", "max"} {
		response = patch(`{"chat":{"floatingLettersDensity":"` + density + `"}}`)
		decodeServerTestJSON(t, response, &gui)
		persisted, err := config.ReadGUISettings()
		if err != nil {
			t.Fatal(err)
		}
		if gui.Chat.FloatingLettersDensity == nil || *gui.Chat.FloatingLettersDensity != density || persisted.Chat.FloatingLettersDensity == nil || *persisted.Chat.FloatingLettersDensity != density {
			t.Fatal("floating letters density did not persist and round trip")
		}
		if persisted.Chat.AutoCloseToolCalls == nil || !*persisted.Chat.AutoCloseToolCalls || persisted.Chat.FloatingLetters == nil || *persisted.Chat.FloatingLetters {
			t.Fatal("density update changed other preferences")
		}
	}
	response = patch(`{"general":{"responseLanguage":" Italian "}}`)
	decodeServerTestJSON(t, response, &gui)
	if gui.General.ResponseLanguage != "Italian" {
		t.Fatalf("language did not round trip: %+v", gui.General)
	}
	document := guiConfigDocument(t, path)
	if document["response_language"] != "Italian" {
		t.Fatal("language was not saved in runtime config")
	}
	if _, exists := document["gui"].(map[string]any)["general"]; exists {
		t.Fatal("language duplicated in GUI config")
	}
	if document["providers"].(map[string]any)["OpenAI"].(map[string]any)["api_key"] != "fixture-secret" {
		t.Fatal("language update changed credentials")
	}
	if gui.Chat.AutoCloseToolCalls == nil || !*gui.Chat.AutoCloseToolCalls {
		t.Fatal("language update changed chat settings")
	}
	response, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	decodeServerTestJSON(t, response, &gui)
	if gui.General.ResponseLanguage != "Italian" {
		t.Fatal("language did not survive reread")
	}
	response = patch(`{"general":{"userName":"New name"},"advanced":{"subagent_timeout_minutes":30,"tool_output.max_lines":512}}`)
	decodeServerTestJSON(t, response, &gui)
	if gui.General.UserName != "New name" || gui.General.ResponseLanguage != "Italian" {
		t.Fatal("name update changed language")
	}
	persisted, err = config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Advanced["subagent_timeout_minutes"] != 30 || persisted.Advanced["tool_output.max_lines"] != 512 {
		t.Fatalf("advanced API values did not persist: %+v", persisted.Advanced)
	}
	directory := filepath.Join(t.TempDir(), "gui")
	if err := os.MkdirAll(filepath.Join(directory, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	response = patch(`{"advanced":{"server.dev_mode":true}}`)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatal("dev mode without a directory was not rejected as bad input")
	}
	body, err := json.Marshal(map[string]any{"advanced": map[string]any{"server.dev_mode": true, "server.dev_directory": directory, "server_port": 64500}})
	if err != nil {
		t.Fatal(err)
	}
	response = patch(string(body))
	decodeServerTestJSON(t, response, &gui)
	if gui.Advanced["server.dev_mode"] != true || gui.Advanced["server.dev_directory"] != directory || gui.Advanced["server_port"] != float64(64500) {
		t.Fatalf("missing server API values: %#v", gui.Advanced)
	}
	persisted, err = config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Advanced["server.dev_mode"] != true || persisted.Advanced["server_port"] != 64500 {
		t.Fatal("server settings did not persist")
	}
	// The API server remains on its original address until restarted.
	response, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	decodeServerTestJSON(t, response, &gui)
	for _, body := range []string{`{"advanced":{"server_port":0}}`, `{"advanced":{"server.dev_mode":null}}`, `{"advanced":{"server.dev_directory":""}}`, `{"advanced":{"server.dev_directory":"/missing/gui"}}`, `{"advanced":{"subagent_timeout_minutes":0}}`, `{"advanced":{"log_level":"invalid"}}`, `{"advanced":{"api_key":"secret"}}`, `{"general":{"userName":null}}`, `{"general":{"userName":" "}}`, `{"general":{}}`, `{"general":{"responseLanguage":null}}`, `{"general":{"responseLanguage":" "}}`, `{"general":{"responseLanguage":"Italian\nEnglish"}}`, `{"general":{"responseLanguage":true}}`, `{"chat":{}}`, `{"chat":{"autoCloseToolCalls":"true"}}`, `{"chat":{"autoCloseToolCalls":null}}`, `{"chat":{"startReasoningCollapsed":"true"}}`, `{"chat":{"startReasoningCollapsed":null}}`, `{"chat":{"floatingLetters":null}}`, `{"chat":{"floatingLetters":"true"}}`, `{"chat":{"floatingLettersDensity":"ultra"}}`, `{"chat":{"floatingLettersDensity":""}}`, `{"chat":{"floatingLettersDensity":true}}`, `{"chat":{"floatingLettersDensity":2}}`, `{"chat":{"floatingLettersDensity":null}}`} {
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
