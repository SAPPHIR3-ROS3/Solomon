package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type sseTransformer struct {
	model             string
	responseID        string
	roleSent          bool
	toolIndexByItemID map[string]int
	toolIDByItemID    map[string]string
	nextToolIndex     int
	sawToolCalls      bool
}

func newSSETransformer(model string) *sseTransformer {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-5.4"
	}
	return &sseTransformer{
		model:             model,
		toolIndexByItemID: make(map[string]int),
		toolIDByItemID:    make(map[string]string),
	}
}

func (t *sseTransformer) transform(dataLine []byte) (out []byte, done bool, err error) {
	trimmed := bytes.TrimSpace(dataLine)
	if len(trimmed) == 0 {
		return nil, false, nil
	}
	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, true, nil
	}
	var upstream map[string]any
	if err := json.Unmarshal(trimmed, &upstream); err != nil {
		return nil, false, fmt.Errorf("invalid codex SSE JSON: %w", err)
	}
	eventType, _ := upstream["type"].(string)
	sendRole := func(seq any) ([]byte, error) {
		if t.roleSent {
			return nil, nil
		}
		roleChunk := map[string]any{
			"id": t.responseID, "object": "chat.completion.chunk", "created": seq, "model": t.model,
			"choices": []any{map[string]any{
				"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil,
			}},
		}
		b, err := json.Marshal(roleChunk)
		if err != nil {
			return nil, err
		}
		t.roleSent = true
		return b, nil
	}
	if strings.HasPrefix(eventType, "response.reasoning") {
		if idx, ok := upstream["output_index"].(float64); ok && idx > 0 {
			return nil, false, nil
		}
		if !strings.Contains(eventType, ".delta") {
			return nil, false, nil
		}
		reasoningText := extractReasoningContent(upstream)
		if reasoningText == "" {
			return nil, false, nil
		}
		var chunks [][]byte
		if rb, err := sendRole(upstream["sequence_number"]); err != nil {
			return nil, false, err
		} else if len(rb) > 0 {
			chunks = append(chunks, rb)
		}
		reasoningChunk := map[string]any{
			"id": t.responseID, "object": "chat.completion.chunk", "created": upstream["sequence_number"], "model": t.model,
			"choices": []any{map[string]any{
				"index": 0, "delta": map[string]any{"reasoning_content": reasoningText}, "finish_reason": nil,
			}},
		}
		b, err := json.Marshal(reasoningChunk)
		if err != nil {
			return nil, false, err
		}
		chunks = append(chunks, b)
		return bytes.Join(chunks, []byte("\n")), false, nil
	}
	switch eventType {
	case "response.created":
		if resp, ok := upstream["response"].(map[string]any); ok {
			if id, ok := resp["id"].(string); ok {
				t.responseID = "chatcmpl-" + id
			}
		}
		return nil, false, nil
	case "response.output_item.added":
		item, _ := upstream["item"].(map[string]any)
		if item == nil || item["type"] != "function_call" {
			return nil, false, nil
		}
		fcID, _ := item["id"].(string)
		callID, _ := item["call_id"].(string)
		name, _ := item["name"].(string)
		idx, ok := t.toolIndexByItemID[fcID]
		if !ok {
			idx = t.nextToolIndex
			t.nextToolIndex++
			t.toolIndexByItemID[fcID] = idx
		}
		if callID == "" {
			callID = "call_" + fcID
		}
		t.toolIDByItemID[fcID] = callID
		t.sawToolCalls = true
		var chunks [][]byte
		if rb, err := sendRole(upstream["sequence_number"]); err != nil {
			return nil, false, err
		} else if len(rb) > 0 {
			chunks = append(chunks, rb)
		}
		toolStart := map[string]any{
			"id": t.responseID, "object": "chat.completion.chunk", "created": upstream["sequence_number"], "model": t.model,
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"tool_calls": []any{map[string]any{
					"index": idx, "id": callID, "type": "function",
					"function": map[string]any{"name": name, "arguments": ""},
				}}},
				"finish_reason": nil,
			}},
		}
		b, err := json.Marshal(toolStart)
		if err != nil {
			return nil, false, err
		}
		chunks = append(chunks, b)
		return bytes.Join(chunks, []byte("\n")), false, nil
	case "response.function_call_arguments.delta":
		itemID, _ := upstream["item_id"].(string)
		idx, ok := t.toolIndexByItemID[itemID]
		if !ok {
			return nil, false, nil
		}
		argDelta, _ := upstream["delta"].(string)
		var chunks [][]byte
		if rb, err := sendRole(upstream["sequence_number"]); err != nil {
			return nil, false, err
		} else if len(rb) > 0 {
			chunks = append(chunks, rb)
		}
		toolArgs := map[string]any{
			"id": t.responseID, "object": "chat.completion.chunk", "created": upstream["sequence_number"], "model": t.model,
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"tool_calls": []any{map[string]any{
					"index": idx, "function": map[string]any{"arguments": argDelta},
				}}},
				"finish_reason": nil,
			}},
		}
		b, err := json.Marshal(toolArgs)
		if err != nil {
			return nil, false, err
		}
		chunks = append(chunks, b)
		return bytes.Join(chunks, []byte("\n")), false, nil
	case "response.output_text.delta":
		var chunks [][]byte
		if rb, err := sendRole(upstream["sequence_number"]); err != nil {
			return nil, false, err
		} else if len(rb) > 0 {
			chunks = append(chunks, rb)
		}
		delta, _ := upstream["delta"].(string)
		contentChunk := map[string]any{
			"id": t.responseID, "object": "chat.completion.chunk", "created": upstream["sequence_number"], "model": t.model,
			"choices": []any{map[string]any{
				"index": 0, "delta": map[string]any{"content": delta}, "finish_reason": nil,
			}},
		}
		b, err := json.Marshal(contentChunk)
		if err != nil {
			return nil, false, err
		}
		chunks = append(chunks, b)
		return bytes.Join(chunks, []byte("\n")), false, nil
	case "response.completed":
		finish := "stop"
		if t.sawToolCalls {
			finish = "tool_calls"
		}
		var usage map[string]any
		if respObj, ok := upstream["response"].(map[string]any); ok {
			if u, ok := respObj["usage"].(map[string]any); ok {
				outUsage := map[string]any{}
				if it, ok := u["input_tokens"].(float64); ok {
					outUsage["prompt_tokens"] = int(it)
				}
				if ot, ok := u["output_tokens"].(float64); ok {
					outUsage["completion_tokens"] = int(ot)
				}
				if tt, ok := u["total_tokens"].(float64); ok {
					outUsage["total_tokens"] = int(tt)
				} else if pt, ok := outUsage["prompt_tokens"].(int); ok {
					if ct, ok2 := outUsage["completion_tokens"].(int); ok2 {
						outUsage["total_tokens"] = pt + ct
					}
				}
				if len(outUsage) > 0 {
					usage = outUsage
				}
			}
		}
		if usage == nil {
			usage = map[string]any{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
		}
		finalChunk := map[string]any{
			"id": t.responseID, "object": "chat.completion.chunk", "created": upstream["sequence_number"], "model": t.model,
			"choices": []any{map[string]any{
				"index": 0, "delta": map[string]any{}, "finish_reason": finish,
			}},
			"usage": usage,
		}
		b, err := json.Marshal(finalChunk)
		if err != nil {
			return nil, false, err
		}
		return b, false, nil
	default:
		return nil, false, nil
	}
}

func extractReasoningContent(evt map[string]any) string {
	if delta, _ := evt["delta"].(string); delta != "" {
		return delta
	}
	if text, _ := evt["text"].(string); text != "" {
		return text
	}
	if part, ok := evt["part"].(map[string]any); ok {
		if t, _ := part["text"].(string); t != "" {
			return t
		}
	}
	if item, ok := evt["item"].(map[string]any); ok {
		if encrypted, ok := item["encrypted_content"].(string); ok && encrypted != "" {
			return ""
		}
		if summaryArr, ok := item["summary"].([]any); ok {
			for _, entry := range summaryArr {
				if sm, ok := entry.(map[string]any); ok {
					if t, _ := sm["text"].(string); t != "" {
						return t
					}
				}
			}
		}
	}
	return ""
}
