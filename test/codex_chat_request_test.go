package test

import (
	"encoding/json"
	"testing"

	codexchat "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex/chat"
)

func TestBuildCodexInputUserImages(t *testing.T) {
	imgURL := "data:image/png;base64,abc"
	chat := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "before"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": imgURL}},
					map[string]any{"type": "text", "text": "after"},
				},
			},
		},
	}
	input := codexchat.BuildCodexInput(chat)
	if len(input) != 1 {
		t.Fatalf("want 1 input item, got %d", len(input))
	}
	msg, ok := input[0].(map[string]any)
	if !ok || msg["role"] != "user" {
		t.Fatalf("unexpected message: %#v", input[0])
	}
	contents, ok := msg["content"].([]any)
	if !ok || len(contents) != 3 {
		t.Fatalf("want 3 content parts, got %#v", msg["content"])
	}
	text0, _ := contents[0].(map[string]any)
	if text0["type"] != "input_text" || text0["text"] != "before" {
		t.Fatalf("first part: %#v", contents[0])
	}
	img, _ := contents[1].(map[string]any)
	if img["type"] != "input_image" || img["image_url"] != imgURL {
		t.Fatalf("image part: %#v", contents[1])
	}
	text2, _ := contents[2].(map[string]any)
	if text2["type"] != "input_text" || text2["text"] != "after" {
		t.Fatalf("third part: %#v", contents[2])
	}
}

func TestBuildCodexInputSkipsAssistantToolCallsWithEmptyID(t *testing.T) {
	chat := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{
				"role": "assistant",
				"tool_calls": []any{
					map[string]any{
						"id": "",
						"type": "function",
						"function": map[string]any{"name": "shell", "arguments": "{}"},
					},
				},
			},
		},
	}
	input := codexchat.BuildCodexInput(chat)
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "function_call" {
			t.Fatalf("unexpected function_call with empty id: %#v", m)
		}
	}
}

func TestBuildCodexInputImageOnlyUserMessage(t *testing.T) {
	imgURL := "data:image/jpeg;base64,xyz"
	chat := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": imgURL}},
				},
			},
		},
	}
	input := codexchat.BuildCodexInput(chat)
	if len(input) != 1 {
		t.Fatalf("want image-only user message preserved, got %d items", len(input))
	}
	body, err := codexchat.ChatCompletionToCodexBody(chat)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	rawInput, ok := decoded["input"].([]any)
	if !ok || len(rawInput) != 1 {
		t.Fatalf("codex body input: %#v", decoded["input"])
	}
}
