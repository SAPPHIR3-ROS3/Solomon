package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

func signatureSwitchMode(mode string) {}

type switchModeArgs struct {
	Mode string `json:"mode"`
}

func switchModeOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("switchMode", "Switch session mode between agent (implementation) and chat (web/docs). A 5-second countdown runs before the switch; press Ctrl+C to cancel.", map[string]any{
		"mode": map[string]any{"type": "string", "enum": []string{"agent", "chat"}, "description": "Target mode"},
	}, []string{"mode"})
}

func appendSwitchModeDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureSwitchMode)
	if err != nil {
		return err
	}
	b.addBlock("switchMode", "Switch between agent and chat modes after a cancellable countdown.", sig)
	return nil
}

func execSwitchMode(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a switchModeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(strings.ToLower(a.Mode))
	if target != "agent" && target != "chat" {
		return nil, fmt.Errorf("switchMode: mode must be agent or chat")
	}
	cur := "agent"
	if env.CurrentMode != nil {
		cur = env.CurrentMode()
	}
	if EffectiveSurfaceMode(cur) == target {
		return map[string]any{"ok": true, "mode": target, "unchanged": true}, nil
	}
	if env.SwitchModeCountdown == nil {
		if env.SetMode != nil {
			env.SetMode(target)
		}
		return map[string]any{"ok": true, "mode": target}, nil
	}
	cancelled, err := env.SwitchModeCountdown(ctx, target)
	if err != nil {
		return nil, err
	}
	if cancelled {
		return map[string]any{"ok": false, "cancelled": true, "mode": EffectiveSurfaceMode(cur)}, nil
	}
	return map[string]any{"ok": true, "mode": target}, nil
}
