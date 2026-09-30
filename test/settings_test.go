package test

import (
	"context"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"os"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestSettingsValidatedPersistence(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	logging.LogInit(logging.INFO_LOG_LEVEL)
	if err := logging.Configure(logging.Config{Dir: t.TempDir(), WriteFile: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logging.Configure(logging.Config{WriteConsole: true, WriteFile: false}) })
	root := config.EmptyRoot()
	root.ResponseLanguage = "English"
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	path, _ := paths.ConfigPath()
	before, _ := os.ReadFile(path)
	for _, tc := range []struct{ key, value, expected string }{
		{"response_language", `null`, `"English"`},
		{"response_language", `"Italian"`, `"French"`},
		{"subagent_timeout_minutes", `181`, `20`},
		{"show_thinking", `"true"`, `false`},
		{"web_search_api_key", `"secret"`, `null`},
		{"reasoning_effort", `"typo"`, `"none"`},
	} {
		if _, err := config.UpdateSetting(tc.key, json.RawMessage(tc.value), json.RawMessage(tc.expected), nil); err == nil {
			t.Fatalf("accepted invalid update: %+v", tc)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatal("invalid update changed file")
		}
	}
	updated, err := config.UpdateSetting("response_language", json.RawMessage(`"Italian"`), json.RawMessage(`"English"`), nil)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := logging.ReadLast(20)
	if err != nil {
		t.Fatal(err)
	}
	audit := strings.Join(lines, "\n")
	for _, want := range []string{"setting updated", "before:English", "after:Italian"} {
		if !strings.Contains(audit, want) {
			t.Fatalf("missing audit %q: %s", want, audit)
		}
	}
	if updated.ResponseLanguage != "Italian" {
		t.Fatal("update missing")
	}
	loaded, err := config.Load()
	if err != nil || loaded.ResponseLanguage != "Italian" {
		t.Fatalf("persistence: %v", err)
	}
	for _, mode := range []string{"agent", "chat"} {
		_, err := tools.Exec(context.Background(), &tools.Env{}, mode, tooling.Invocation{Name: "settings", Args: json.RawMessage(`{"action":"read","intent":"Read current settings"}`)})
		if err != nil {
			t.Fatal(err)
		}
		_, err = tools.Exec(context.Background(), &tools.Env{}, mode, tooling.Invocation{Name: "settings", Args: json.RawMessage(`{"action":"update","key":"response_language","value":"French","expected":"Italian","extra":true,"intent":"Update response language"}`)})
		if err == nil {
			t.Fatal("accepted unknown argument")
		}
	}
}
