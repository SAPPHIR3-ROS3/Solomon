package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/guicustom"
	"github.com/openai/openai-go/v2"
)

const customizeUIDescription = "Use when the user asks to personalize Solomon's own interface. action=dev automatically exports editable sources matching the installed version, installs frontend dependencies, and activates hot reload without restarting the daemon or interrupting chats/terminals. Optional directory reuses existing GUI sources. Edit the returned dev_directory with file tools to implement the requested design. Never stop/restart Solomon from its own chat. action=normal restores the bundled interface while preserving custom files; action=status inspects the active frontend. Requires Node.js and npm."

func customizeUIOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("customizeUI", customizeUIDescription, map[string]any{
		"action":    map[string]any{"type": "string", "enum": []string{"dev", "normal", "status"}},
		"directory": map[string]any{"type": "string", "description": "Optional existing GUI directory or empty folder to initialize from the installed interface; omit for automatic setup"},
	}, []string{"action"})
}

func execCustomizeUI(ctx context.Context, raw json.RawMessage) (any, error) {
	var args struct {
		Action    string `json:"action"`
		Directory string `json:"directory"`
		Intent    string `json:"intent"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	if args.Action != "dev" && args.Action != "normal" && args.Action != "status" {
		return nil, fmt.Errorf("action must be dev, normal or status")
	}
	if args.Action != "dev" && args.Directory != "" {
		return nil, fmt.Errorf("directory is only supported for action=dev")
	}
	return guicustom.Request(ctx, args.Action, args.Directory)
}
