package anthropic

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/claudecode"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

const (
	claudeCodeIdentityPrompt = "You are Claude Code, Anthropic's official CLI for Claude."
	billingHeaderSalt        = "59cf53e54c78"
)

var billingHeaderPositions = []int{4, 7, 20}

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
			content := m.Content
			if strings.TrimSpace(m.APIContent) != "" {
				content = m.APIContent
			}
			content = chatstore.StripUnresolvedImgPlaceholders(content, imageFiles)
			out = append(out, messageParam{Role: "user", Content: userBlocks(content, imageFiles)})
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
	segs := images.ParseUserContentSegments(content, imageFiles)
	var blocks []contentBlock
	for _, seg := range segs {
		if seg.Text != "" {
			blocks = append(blocks, contentBlock{"type": "text", "text": seg.Text})
		}
		if seg.ImagePath != "" {
			if b := imageBlockFromFile(seg.ImagePath); b != nil {
				blocks = append(blocks, b)
			}
		}
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

func shapeOAuthBody(body map[string]any, req apitype.TurnRequest, auth Auth) {
	if auth.Kind != AuthOAuthBearer {
		return
	}
	messages, _ := body["messages"].([]messageParam)
	body["system"] = buildOAuthSystem(req.System, messages)
	if uid := oauthUserID(auth.Token); uid != "" {
		body["metadata"] = map[string]any{"user_id": uid}
	}
	applyOAuthThinking(body, req)
}

func shapeOAuthSimpleBody(body map[string]any, req apitype.SimpleCompletionRequest, auth Auth) {
	if auth.Kind != AuthOAuthBearer {
		return
	}
	messages, _ := body["messages"].([]messageParam)
	body["system"] = buildOAuthSystem(req.System, messages)
	if uid := oauthUserID(auth.Token); uid != "" {
		body["metadata"] = map[string]any{"user_id": uid}
	}
	applyOAuthThinking(body, apitype.TurnRequest{
		Cfg:                   req.Cfg,
		Model:                 req.Model,
		ForceDisableReasoning: req.ForceDisableReasoning,
	})
}

func buildOAuthSystem(system string, messages []messageParam) []contentBlock {
	blocks := []contentBlock{{"type": "text", "text": claudeCodeIdentityPrompt}}
	if billing := buildBillingHeader(messages); billing != "" {
		blocks = append([]contentBlock{{"type": "text", "text": billing}}, blocks...)
	}
	if s := strings.TrimSpace(system); s != "" {
		blocks = append(blocks, contentBlock{"type": "text", "text": s})
	}
	return blocks
}

func buildBillingHeader(messages []messageParam) string {
	text := firstUserText(messages)
	if text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(text))
	cch := hex.EncodeToString(sum[:])[:5]
	var sampled strings.Builder
	for _, idx := range billingHeaderPositions {
		if idx < len(text) {
			sampled.WriteByte(text[idx])
		} else {
			sampled.WriteByte('0')
		}
	}
	suffixInput := billingHeaderSalt + sampled.String() + claudecode.Version()
	suffixSum := sha256.Sum256([]byte(suffixInput))
	suffix := hex.EncodeToString(suffixSum[:])[:3]
	return "x-anthropic-billing-header: cc_version=" + claudecode.Version() + "." + suffix + "; cc_entrypoint=" + claudeCodeOAuthEntrypoint + "; cch=" + cch + ";"
}

func firstUserText(messages []messageParam) string {
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		for _, block := range msg.Content {
			if typ, _ := block["type"].(string); typ == "text" {
				if text, _ := block["text"].(string); strings.TrimSpace(text) != "" {
					return text
				}
			}
		}
	}
	return ""
}

func oauthUserID(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:16])
}

func applyOAuthThinking(body map[string]any, req apitype.TurnRequest) {
	model := strings.ToLower(strings.TrimSpace(req.Model))
	if strings.Contains(model, "haiku") {
		return
	}
	if req.ForceDisableReasoning {
		body["thinking"] = map[string]any{"type": "disabled"}
		return
	}
	if !modelUsesAdaptiveThinking(model) {
		return
	}
	body["thinking"] = map[string]any{"type": "adaptive", "display": "summarized"}
	if effort := oauthEffortFromConfig(req.Cfg); effort != "" {
		body["output_config"] = map[string]any{"effort": effort}
	}
}

func modelUsesAdaptiveThinking(model string) bool {
	for _, marker := range []string{"4-6", "4-7", "4-8"} {
		if strings.Contains(model, marker) {
			return true
		}
	}
	return false
}

func oauthEffortFromConfig(cfg *config.Root) string {
	if cfg == nil {
		return "high"
	}
	c, err := config.ParseReasoningEffortToken(cfg.ReasoningEffort)
	if err != nil || c == "none" {
		return "high"
	}
	switch c {
	case "low":
		return "low"
	case "med":
		return "medium"
	default:
		return "high"
	}
}
