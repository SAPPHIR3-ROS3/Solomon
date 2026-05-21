package llm

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

type anthropicContentBlock map[string]any

type anthropicMessageParam struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicToolParam struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

func buildAnthropicTools(defs []ToolDef) []anthropicToolParam {
	var out []anthropicToolParam
	for _, d := range defs {
		schema := d.Parameters
		if schema == nil {
			schema = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		if _, ok := schema["type"]; !ok {
			schema["type"] = "object"
		}
		out = append(out, anthropicToolParam{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: schema,
		})
	}
	return out
}

func buildAnthropicMessages(msgs []chatstore.Message, imageFiles map[int]string) []anthropicMessageParam {
	msgs = MessagesForAPI(msgs)
	var out []anthropicMessageParam
	i := 0
	for i < len(msgs) {
		m := msgs[i]
		switch m.Role {
		case "user":
			out = append(out, anthropicMessageParam{Role: "user", Content: anthropicUserBlocks(m.Content, imageFiles)})
			i++
		case "assistant":
			var blocks []anthropicContentBlock
			if c := strings.TrimSpace(m.Content); c != "" {
				blocks = append(blocks, anthropicContentBlock{"type": "text", "text": c})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, anthropicContentBlock{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": parseAnthropicToolInput(tc.Arguments),
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropicContentBlock{"type": "text", "text": ""})
			}
			out = append(out, anthropicMessageParam{Role: "assistant", Content: blocks})
			i++
		case "tool":
			var results []anthropicContentBlock
			for i < len(msgs) && msgs[i].Role == "tool" {
				tm := msgs[i]
				results = append(results, anthropicContentBlock{
					"type":        "tool_result",
					"tool_use_id": tm.ToolCallID,
					"content":     tm.Content,
				})
				i++
			}
			if len(results) > 0 {
				out = append(out, anthropicMessageParam{Role: "user", Content: results})
			} else {
				i++
			}
		default:
			if c := strings.TrimSpace(m.Content); c != "" {
				out = append(out, anthropicMessageParam{Role: "user", Content: []anthropicContentBlock{{"type": "text", "text": m.Role + ": " + c}}})
			}
			i++
		}
	}
	return out
}

func anthropicUserBlocks(content string, imageFiles map[int]string) []anthropicContentBlock {
	content = chatstore.StripUnresolvedImgPlaceholders(content, imageFiles)
	var blocks []anthropicContentBlock
	if !imgPlaceholderRe.MatchString(content) {
		if strings.TrimSpace(content) != "" {
			blocks = append(blocks, anthropicContentBlock{"type": "text", "text": content})
		}
		return blocks
	}
	idx := 0
	for _, m := range imgPlaceholderRe.FindAllStringSubmatchIndex(content, -1) {
		if m[0] > idx {
			blocks = append(blocks, anthropicContentBlock{"type": "text", "text": content[idx:m[0]]})
		}
		seq := atoi(content[m[2]:m[3]])
		idx = m[1]
		if path, ok := imageFiles[seq]; ok {
			if b := anthropicImageBlockFromFile(path); b != nil {
				blocks = append(blocks, b)
			}
		}
	}
	if idx < len(content) {
		blocks = append(blocks, anthropicContentBlock{"type": "text", "text": content[idx:]})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, anthropicContentBlock{"type": "text", "text": ""})
	}
	return blocks
}

func anthropicImageBlockFromFile(path string) anthropicContentBlock {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	mime, ok := imageMIMEForBinary(data)
	if !ok {
		return nil
	}
	return anthropicContentBlock{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": mime,
			"data":       base64.StdEncoding.EncodeToString(data),
		},
	}
}

func parseAnthropicToolInput(args string) map[string]any {
	args = strings.TrimSpace(args)
	if args == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}
