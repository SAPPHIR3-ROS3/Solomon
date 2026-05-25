package codex

import (
	"encoding/json"
	"strings"
)

func ChatCompletionToCodexBody(chat map[string]any) ([]byte, error) {
	model, _ := chat["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-5.4"
	}
	stream, _ := chat["stream"].(bool)
	instructions := extractSystemInstructions(chat)
	input := BuildCodexInput(chat)
	body := map[string]any{
		"model":        model,
		"instructions": instructions,
		"store":        false,
		"stream":       stream,
		"input":        input,
		"tools":        mapToolsToCodex(chat),
		"include":      []any{"reasoning.encrypted_content"},
	}
	if tc, ok := chat["tool_choice"]; ok && tc != nil {
		body["tool_choice"] = tc
	} else {
		body["tool_choice"] = "auto"
	}
	if ptc, ok := chat["parallel_tool_calls"].(bool); ok {
		body["parallel_tool_calls"] = ptc
	} else {
		body["parallel_tool_calls"] = true
	}
	if rs := buildReasoningFromChat(chat); len(rs) > 0 {
		body["reasoning"] = rs
	}
	return json.Marshal(body)
}

func buildReasoningFromChat(chat map[string]any) map[string]any {
	effort := ""
	if e, ok := chat["reasoning_effort"].(string); ok {
		effort = strings.TrimSpace(e)
	}
	if effort == "" || effort == "none" {
		return nil
	}
	out := map[string]any{"summary": "auto"}
	switch strings.ToLower(effort) {
	case "minimal", "low", "medium", "high", "xhigh":
		out["effort"] = strings.ToLower(effort)
	}
	return out
}

func extractSystemInstructions(chat map[string]any) string {
	msgs, _ := chat["messages"].([]any)
	var parts []string
	for _, m := range msgs {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := mm["role"].(string)
		if role != "system" {
			continue
		}
		for _, t := range collectTextSegments(mm["content"], false) {
			parts = append(parts, t)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func BuildCodexInput(chat map[string]any) []any {
	msgs, _ := chat["messages"].([]any)
	var input []any
	for _, m := range msgs {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := mm["role"].(string)
		switch role {
		case "system":
			continue
		case "user":
			contents := collectCodexUserContentParts(mm["content"])
			if len(contents) == 0 {
				continue
			}
			input = append(input, map[string]any{
				"type": "message", "role": "user", "content": contents,
			})
		case "assistant":
			texts := collectTextSegments(mm["content"], false)
			if len(texts) > 0 {
				contents := make([]any, 0, len(texts))
				for _, t := range texts {
					contents = append(contents, map[string]any{"type": "output_text", "text": t})
				}
				input = append(input, map[string]any{
					"type": "message", "role": "assistant", "content": contents,
				})
			}
			if toolCalls, ok := mm["tool_calls"].([]any); ok {
				for _, tc := range toolCalls {
					tcm, ok := tc.(map[string]any)
					if !ok {
						continue
					}
					callID, _ := tcm["id"].(string)
					funcMap, _ := tcm["function"].(map[string]any)
					name, _ := funcMap["name"].(string)
					input = append(input, map[string]any{
						"type": "function_call", "name": name, "call_id": callID,
						"arguments": extractArgumentsString(funcMap["arguments"]),
					})
				}
			}
		case "tool":
			callID, _ := mm["tool_call_id"].(string)
			if callID == "" {
				continue
			}
			input = append(input, map[string]any{
				"type": "function_call_output", "call_id": callID,
				"output": collectToolOutput(mm["content"]),
			})
		}
	}
	return input
}

func mapToolsToCodex(chat map[string]any) []any {
	toolsRaw, ok := chat["tools"].([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(toolsRaw))
	for _, t := range toolsRaw {
		tm, ok := t.(map[string]any)
		if !ok || tm["type"] != "function" {
			continue
		}
		fn, _ := tm["function"].(map[string]any)
		if fn == nil {
			continue
		}
		out = append(out, map[string]any{
			"type": "function", "name": fn["name"], "description": fn["description"],
			"strict": false, "parameters": fn["parameters"],
		})
	}
	return out
}

func collectCodexUserContentParts(content any) []any {
	switch v := content.(type) {
	case string:
		t := strings.TrimSpace(v)
		if t == "" {
			return nil
		}
		return []any{map[string]any{"type": "input_text", "text": t}}
	case []any:
		var parts []any
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if url := chatPartImageURL(m); url != "" {
				parts = append(parts, map[string]any{
					"type":      "input_image",
					"image_url": url,
				})
				continue
			}
			if text, _ := m["text"].(string); text != "" {
				parts = append(parts, map[string]any{"type": "input_text", "text": text})
			}
		}
		return parts
	default:
		return nil
	}
}

func chatPartImageURL(m map[string]any) string {
	switch v := m["image_url"].(type) {
	case map[string]any:
		if url, _ := v["url"].(string); strings.TrimSpace(url) != "" {
			return strings.TrimSpace(url)
		}
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func collectTextSegments(content any, _ bool) []string {
	switch v := content.(type) {
	case string:
		t := strings.TrimSpace(v)
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var texts []string
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := m["text"].(string)
			if text != "" {
				texts = append(texts, text)
			}
		}
		return texts
	default:
		return nil
	}
}

func extractArgumentsString(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return ""
	}
}

func collectToolOutput(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, _ := m["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return ""
	}
}
