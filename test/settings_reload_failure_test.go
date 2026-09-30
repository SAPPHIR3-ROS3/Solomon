package test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestSettingsReloadAvailabilityAndFailure(t *testing.T) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	t.Setenv("SOLOMON_HOME", t.TempDir())
	cfg := config.EmptyRoot()
	cfg.ResponseLanguage = "English"
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	inv := tooling.Invocation{Name: "settings", Args: json.RawMessage(`{"action":"update","key":"response_language","value":"Italian","expected":"English","intent":"Apply requested language"}`)}
	if _, err := tools.Exec(context.Background(), &tools.Env{}, "agent", inv); err == nil {
		t.Fatal("update accepted without live reload support")
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ResponseLanguage != "English" {
		t.Fatal("unavailable reload changed persisted config")
	}
	called := false
	result, err := tools.Exec(context.Background(), &tools.Env{ReloadSettings: func(ctx context.Context, root *config.Root) error {
		called = true
		if root.ResponseLanguage != "Italian" {
			t.Fatal("callback did not receive saved config")
		}
		return errors.New("reload unavailable")
	}}, "agent", inv)
	if err != nil {
		t.Fatal(err)
	}
	response := result.(map[string]any)
	if !called || response["ok"] != false || response["persisted"] != true || response["applied"] != false || response["error"] == nil {
		t.Fatalf("reload failure misreported: %v", response)
	}
	loaded, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ResponseLanguage != "Italian" {
		t.Fatal("persistence result incorrect")
	}
}
