package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

type contentBlock map[string]any

type messageParam struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type toolParam struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

func buildTools(defs []apitype.ToolDef) []toolParam {
	var out []toolParam
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
		out = append(out, toolParam{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: schema,
		})
	}
	return out
}

func buildMessages(msgs []chatstore.Message, imageFiles map[int]string) []messageParam {
	msgs = apitype.MessagesForAPI(msgs)
	var out []messageParam
	i := 0
	for i < len(msgs) {
		m := msgs[i]
		switch m.Role {
		case "user":
			out = append(out, messageParam{Role: "user", Content: userBlocks(m.Content, imageFiles)})
			i++
		case "assistant":
			var blocks []contentBlock
			if c := strings.TrimSpace(chatstore.ScrubLiteralImgPlaceholdersForAPI(m.Content)); c != "" {
				blocks = append(blocks, contentBlock{"type": "text", "text": c})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, contentBlock{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": parseToolInput(chatstore.ScrubLiteralImgPlaceholdersForAPI(tc.Arguments)),
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, contentBlock{"type": "text", "text": ""})
			}
			out = append(out, messageParam{Role: "assistant", Content: blocks})
			i++
		case "tool":
			var results []contentBlock
			for i < len(msgs) && msgs[i].Role == "tool" {
				tm := msgs[i]
				results = append(results, contentBlock{
					"type":        "tool_result",
					"tool_use_id": tm.ToolCallID,
					"content":     chatstore.ScrubLiteralImgPlaceholdersForAPI(tm.Content),
				})
				i++
			}
			if len(results) > 0 {
				out = append(out, messageParam{Role: "user", Content: results})
			} else {
				i++
			}
		default:
			if c := strings.TrimSpace(m.Content); c != "" {
				out = append(out, messageParam{Role: "user", Content: []contentBlock{{"type": "text", "text": m.Role + ": " + c}}})
			}
			i++
		}
	}
	return out
}

func userBlocks(content string, imageFiles map[int]string) []contentBlock {
	content = chatstore.StripUnresolvedImgPlaceholders(content, imageFiles)
	var blocks []contentBlock
	if !images.PlaceholderRE.MatchString(content) {
		if strings.TrimSpace(content) != "" {
			blocks = append(blocks, contentBlock{"type": "text", "text": content})
		}
		return blocks
	}
	idx := 0
	for _, m := range images.PlaceholderRE.FindAllStringSubmatchIndex(content, -1) {
		if m[0] > idx {
			blocks = append(blocks, contentBlock{"type": "text", "text": content[idx:m[0]]})
		}
		seq := images.Atoi(content[m[2]:m[3]])
		idx = m[1]
		if path, ok := imageFiles[seq]; ok {
			if b := imageBlockFromFile(path); b != nil {
				blocks = append(blocks, b)
			}
		}
	}
	if idx < len(content) {
		blocks = append(blocks, contentBlock{"type": "text", "text": content[idx:]})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, contentBlock{"type": "text", "text": ""})
	}
	return blocks
}

func imageBlockFromFile(path string) contentBlock {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	mime, ok := images.MIMEForBinary(data)
	if !ok {
		return nil
	}
	return contentBlock{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": mime,
			"data":       base64.StdEncoding.EncodeToString(data),
		},
	}
}

func parseToolInput(args string) map[string]any {
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
