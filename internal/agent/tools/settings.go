package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/openai/openai-go/v2"
)

func settingsOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("settings", "Read or update Solomon settings. Always read first: returns supported keys, types, limits and persisted values (null means default). Update one key with value and expected equal to its last read value. Use only for user-requested changes. Never edit the config file through shell/editFile. Credentials, providers and advanced settings are excluded. Changes are persisted and automatically applied to the active session before the next model request. In-flight requests keep their previous settings.", map[string]any{
		"action":   map[string]any{"type": "string", "enum": []string{"read", "update"}},
		"key":      map[string]any{"type": "string"},
		"value":    map[string]any{"type": []string{"string", "integer", "boolean", "null"}},
		"expected": map[string]any{"type": []string{"string", "integer", "boolean", "null"}},
	}, []string{"action"})
}

func execSettings(ctx context.Context, env *Env, raw json.RawMessage, audit map[string]any) (any, error) {
	var args struct {
		Action   string          `json:"action"`
		Key      string          `json:"key"`
		Value    json.RawMessage `json:"value"`
		Expected json.RawMessage `json:"expected"`
		Intent   string          `json:"intent"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON object")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch args.Action {
	case "read":
		if args.Key != "" || len(args.Value) > 0 || len(args.Expected) > 0 {
			return nil, fmt.Errorf("read accepts only action and intent")
		}
		root, err := config.Load()
		if err != nil {
			return nil, err
		}
		return map[string]any{"values": config.SettingsValues(root), "supported": config.SettingsSpecs()}, nil
	case "update":
		if env == nil || env.ReloadSettings == nil {
			return nil, fmt.Errorf("live settings reload unavailable; configuration was not changed")
		}
		root, err := config.UpdateSetting(args.Key, args.Value, args.Expected, audit)
		if err != nil {
			return nil, err
		}
		if err := env.ReloadSettings(ctx, root); err != nil {
			return map[string]any{"ok": false, "persisted": true, "applied": false, "key": args.Key, "error": fmt.Sprintf("settings saved but live reload failed: %v", err)}, nil
		}
		return map[string]any{"ok": true, "key": args.Key, "value": config.SettingsValues(root)[args.Key], "applied": true, "restart_required": false}, nil
	default:
		return nil, fmt.Errorf("action must be read or update")
	}
}
