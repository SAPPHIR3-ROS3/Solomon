package test

import (
	"encoding/json"
	"strings"
	"testing"

	codexchat "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex/chat"
)

func TestBuildCodexInputLongCallIDs(t *testing.T) {
	prefix := strings.Repeat("a", 64)
	ids := []string{"call_short", prefix, prefix + strings.Repeat("b", 19), prefix + strings.Repeat("c", 19)}
	var messages []any
	for _, id := range ids {
		messages = append(messages,
			map[string]any{"role": "assistant", "tool_calls": []any{
				map[string]any{"id": id, "function": map[string]any{"name": "shell", "arguments": "{}"}},
			}},
			map[string]any{"role": "tool", "tool_call_id": id, "content": "done"},
		)
	}
	input := codexchat.BuildCodexInput(map[string]any{"messages": messages})
	if len(input) != 2*len(ids) {
		t.Fatalf("unexpected input count: %d", len(input))
	}
	seen := map[string]bool{}
	for i, original := range ids {
		call := input[2*i].(map[string]any)["call_id"].(string)
		output := input[2*i+1].(map[string]any)["call_id"].(string)
		if len(call) > 64 || call != output || seen[call] {
			t.Fatalf("invalid call/output mapping: %q / %q", call, output)
		}
		if len(original) <= 64 && call != original {
			t.Fatalf("valid ID changed: %q", original)
		}
		seen[call] = true
	}
	if messages[4].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)["id"] != ids[2] {
		t.Fatal("stored history was modified")
	}
}

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
						"id":       "",
						"type":     "function",
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

func TestBuildCodexInputReplaysEncryptedReasoning(t *testing.T) {
	chat := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "go"},
			map[string]any{"role": "assistant", "content": "done", "reasoning_encrypted_content": "enc-1"},
		},
	}
	input := codexchat.BuildCodexInput(chat)
	if len(input) != 3 {
		t.Fatalf("want user, reasoning, assistant; got %#v", input)
	}
	reasoning, _ := input[1].(map[string]any)
	if summary, ok := reasoning["summary"].([]any); reasoning["type"] != "reasoning" || reasoning["encrypted_content"] != "enc-1" || !ok || len(summary) != 0 {
		t.Fatalf("reasoning item: %#v", input[1])
	}
	full := map[string]any{
		"messages": []any{
			map[string]any{"role": "assistant", "content": "done", "reasoning_item": `{"type":"reasoning","id":"rs_1","encrypted_content":"enc-2","summary":[]}`},
		},
	}
	replayed := codexchat.BuildCodexInput(full)
	item, _ := replayed[0].(map[string]any)
	if item["id"] != "rs_1" || item["encrypted_content"] != "enc-2" {
		t.Fatalf("full reasoning item: %#v", replayed[0])
	}
	// Validate the wire representation: Responses requires summary even when
	// only encrypted reasoning is retained. It must be [] rather than null.
	body, err := codexchat.ChatCompletionToCodexBody(chat)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Input []map[string]json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire.Input[1]["summary"]) != "[]" {
		t.Fatalf("reasoning summary must be an empty array, got %s", wire.Input[1]["summary"])
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
	chat["prompt_cache_key"] = " chat-1 "
	body, err := codexchat.ChatCompletionToCodexBody(chat)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["prompt_cache_key"] != "chat-1" {
		t.Fatalf("prompt_cache_key: %#v", decoded["prompt_cache_key"])
	}
	if _, ok := decoded["prompt_cache_options"]; ok {
		t.Fatal("Codex cache affinity is the session header, not prompt_cache_options")
	}
	rawInput, ok := decoded["input"].([]any)
	if !ok || len(rawInput) != 1 {
		t.Fatalf("codex body input: %#v", decoded["input"])
	}
}

func TestCodexSessionIDFollowsPromptCacheKey(t *testing.T) {
	chat := map[string]any{"prompt_cache_key": " chat-1 "}
	if got := codexchat.CodexSessionID(chat); got != "chat-1" {
		t.Fatalf("session id: %q", got)
	}
	body, err := codexchat.ChatCompletionToCodexBody(map[string]any{
		"model":            "gpt-6.1-sol",
		"prompt_cache_key": " chat-1 ",
		"messages":         []any{map[string]any{"role": "user", "content": "first"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["prompt_cache_key"] != "chat-1" {
		t.Fatalf("prompt_cache_key: %#v", decoded["prompt_cache_key"])
	}
	if _, ok := decoded["prompt_cache_options"]; ok {
		t.Fatal("gpt-6.1-sol follows Codex and must not send prompt_cache_options")
	}
}
