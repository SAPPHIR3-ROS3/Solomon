package test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/toolbatch"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

func TestSettingsToolAppliesLiveSnapshot(t *testing.T) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	t.Setenv("SOLOMON_HOME", t.TempDir())
	initial := config.EmptyRoot()
	initial.ResponseLanguage = "English"
	initial.Current = config.Current{Provider: "active", Model: "model"}
	if err := config.Save(initial); err != nil {
		t.Fatal(err)
	}
	r := &agentruntime.Runtime{Cfg: initial, Model: "model", Out: io.Discard, Mode: "agent", CompactionThresholdTokens: config.EffectiveCompactionThresholdTokens(initial)}
	update := func(key string, value any) {
		t.Helper()
		old := r.Cfg
		before := config.SettingsValues(old)[key]
		raw, _ := json.Marshal(map[string]any{"action": "update", "key": key, "value": value, "expected": before, "intent": "Apply requested setting"})
		result, err := tools.Exec(context.Background(), r.ToolEnvForTest(), "agent", tooling.Invocation{Name: "settings", Args: raw, ToolCallID: "settings-live"})
		if err != nil {
			t.Fatal(err)
		}
		response := result.(map[string]any)
		if response["ok"] != true || response["applied"] != true || response["restart_required"] != false {
			t.Fatalf("unexpected response: %v", response)
		}
		if r.Cfg == old || config.SettingsValues(old)[key] != before {
			t.Fatal("old request snapshot was modified")
		}
		loaded, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		if config.SettingsValues(loaded)[key] != config.SettingsValues(r.Cfg)[key] {
			t.Fatal("live setting differs from persisted value")
		}
	}
	update("response_language", "Italian")
	prompt, err := r.SystemPromptForTest(false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Italian") {
		t.Fatal("next system prompt did not use new language")
	}
	update("compaction_threshold_tokens", int64(65536))
	if r.CompactionThresholdTokens != 65536 {
		t.Fatal("cached compaction threshold not reloaded")
	}
	update("max_response_tokens", 1234)
	var params openai.ChatCompletionNewParams
	llm.ApplyMaxResponseTokens(r.Cfg, &params)
	if params.MaxCompletionTokens.Value != 1234 {
		t.Fatal("next request did not use new token limit")
	}
	if r.Model != "model" || r.Cfg.Current.Provider != "active" {
		t.Fatal("reload changed active model/provider")
	}
	old := r.Cfg
	_, err = tools.Exec(context.Background(), r.ToolEnvForTest(), "agent", tooling.Invocation{Name: "settings", Args: json.RawMessage(`{"action":"update","key":"response_language","value":"French","expected":"English","intent":"Check stale update"}`)})
	if err == nil || r.Cfg != old {
		t.Fatal("stale update changed runtime")
	}
	if toolbatch.CanRunConcurrently([]tooling.Invocation{{Name: "readFile"}, {Name: "settings"}}) {
		t.Fatal("settings must serialize sibling tools")
	}
}

func TestSettingsReloadRefreshesCursorBackend(t *testing.T) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	cfg := config.EmptyRoot()
	p := &config.Provider{Name: config.ProviderNameCursorSub, AuthKind: config.AuthKindOAuthCursor}
	backend, err := llm.NewCompletionBackend(context.Background(), cfg, p)
	if err != nil {
		t.Fatal(err)
	}
	r := &agentruntime.Runtime{Cfg: cfg, Prov: p, Backend: backend}
	next := *cfg
	fast := false
	next.FastMode = &fast
	if err := r.ToolEnvForTest().ReloadSettings(context.Background(), &next); err != nil {
		t.Fatal(err)
	}
	if r.Backend == backend || r.Cfg.EffectiveFastMode() {
		t.Fatal("Cursor backend was not refreshed")
	}
	if !cfg.EffectiveFastMode() {
		t.Fatal("in-flight snapshot changed")
	}
}
