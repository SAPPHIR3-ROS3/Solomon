package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/streamio"
)

var cursorKeyMu sync.Mutex

func (b *cursorSubBackend) cursorAgentKey(ctx context.Context) (string, error) {
	cursorKeyMu.Lock()
	defer cursorKeyMu.Unlock()
	if key := strings.TrimSpace(b.provider.APIKey); strings.HasPrefix(key, "crsr_") || strings.HasPrefix(key, "key_") {
		return key, nil
	}
	return b.mintCursorAgentKey(ctx)
}

func (b *cursorSubBackend) renewCursorAgentKey(ctx context.Context) (string, error) {
	cursorKeyMu.Lock()
	defer cursorKeyMu.Unlock()
	return b.mintCursorAgentKey(ctx)
}

func (b *cursorSubBackend) mintCursorAgentKey(ctx context.Context) (string, error) {
	session, err := config.ResolveCursorSessionBearer(ctx, b.cfg, b.provider)
	if err != nil {
		return "", err
	}
	key, err := cursorauth.MintUserAPIKey(ctx, session)
	if err != nil {
		return "", fmt.Errorf("Cursor subscription API key: %w", err)
	}
	b.provider.APIKey = key
	if b.cfg != nil {
		if err := config.Save(b.cfg); err != nil {
			return "", fmt.Errorf("save Cursor subscription API key: %w", err)
		}
	}
	return key, nil
}

type cursorPendingTool struct {
	session *cursorauth.AgentSession
	call    cursorauth.AgentToolCall
}

func agentToolsFromDefs(defs []ToolDef) ([]cursorauth.AgentTool, error) {
	tools := make([]cursorauth.AgentTool, 0, len(defs))
	for _, def := range defs {
		if strings.TrimSpace(def.Name) == "" {
			continue
		}
		schema := def.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		if len(def.Required) > 0 {
			copySchema := make(map[string]any, len(schema)+1)
			for key, value := range schema {
				copySchema[key] = value
			}
			copySchema["required"] = def.Required
			schema = copySchema
		}
		encoded, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("Cursor tool %s schema: %w", def.Name, err)
		}
		tools = append(tools, cursorauth.AgentTool{Name: def.Name, Description: def.Description, Schema: encoded})
	}
	return tools, nil
}

func cursorImage(path string) (cursorauth.AgentImage, bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 12<<20 {
		return cursorauth.AgentImage{}, false
	}
	mime, ok := images.MIMEForBinary(data)
	if !ok {
		return cursorauth.AgentImage{}, false
	}
	return cursorauth.AgentImage{Data: data, Mime: mime, Filename: filepath.Base(path)}, true
}

func latestCursorUser(messages []chatstore.Message, imageFiles map[int]string) (string, []cursorauth.AgentImage) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		content := messages[i].APIContent
		if content == "" {
			content = messages[i].Content
		}
		content = chatstore.StripUnresolvedImgPlaceholders(content, imageFiles)
		var text strings.Builder
		var media []cursorauth.AgentImage
		for _, segment := range images.ParseUserContentSegments(content, imageFiles) {
			text.WriteString(segment.Text)
			if segment.ImagePath != "" {
				if image, ok := cursorImage(segment.ImagePath); ok {
					media = append(media, image)
				}
			}
		}
		return strings.TrimSpace(text.String()), media
	}
	return "", nil
}

func agentHistory(messages []chatstore.Message, imageFiles map[int]string) ([][]byte, error) {
	lastUser := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUser = i
			break
		}
	}
	if lastUser < 0 {
		return nil, nil
	}
	history := make([][]byte, 0, lastUser)
	toolNames := map[string]string{}
	for _, msg := range messages[:lastUser] {
		var parts []any
		content := msg.APIContent
		if content == "" {
			content = msg.Content
		}
		if msg.Role == "user" {
			content = chatstore.StripUnresolvedImgPlaceholders(content, imageFiles)
			for _, segment := range images.ParseUserContentSegments(content, imageFiles) {
				if segment.Text != "" {
					parts = append(parts, map[string]any{"type": "text", "text": segment.Text})
				}
				if segment.ImagePath != "" {
					if image, ok := cursorImage(segment.ImagePath); ok {
						parts = append(parts, map[string]any{"type": "image", "image": base64.StdEncoding.EncodeToString(image.Data), "mimeType": image.Mime})
					}
				}
			}
		} else if content != "" {
			parts = append(parts, map[string]any{"type": "text", "text": content})
		}
		if msg.Role == "assistant" {
			for _, call := range msg.ToolCalls {
				var args map[string]any
				if json.Unmarshal([]byte(call.Arguments), &args) != nil || args == nil {
					args = map[string]any{}
				}
				parts = append(parts, map[string]any{"type": "tool-call", "toolCallId": call.ID, "toolName": call.Name, "args": args})
				toolNames[call.ID] = call.Name
			}
		}
		if msg.Role == "tool" {
			name := toolNames[msg.ToolCallID]
			if name == "" {
				name = "tool"
			}
			parts = []any{map[string]any{"type": "tool-result", "toolCallId": msg.ToolCallID, "toolName": name, "result": map[string]any{"is_error": false, "content": msg.Content}}}
		}
		if len(parts) == 0 {
			continue
		}
		wire := map[string]any{"role": msg.Role, "content": parts}
		if msg.Role == "tool" {
			wire["id"] = msg.ToolCallID
		}
		encoded, err := json.Marshal(wire)
		if err != nil {
			return nil, err
		}
		history = append(history, encoded)
	}
	return history, nil
}

func cursorToolResult(messages []chatstore.Message, id string) (string, bool, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "tool" && messages[i].ToolCallID == id {
			content := messages[i].Content
			var payload map[string]any
			_ = json.Unmarshal([]byte(content), &payload)
			failed, _ := payload["is_error"].(bool)
			if message, ok := payload["error"].(string); ok && message != "" {
				failed = true
			}
			return content, failed, true
		}
	}
	return "", false, false
}

func (b *cursorSubBackend) selectAgentModel(ctx context.Context, selected, effort string, fast bool) string {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		selected = "composer-2.5"
	}
	resolved := cursorauth.ResolveChatModelID(selected, effort, fast)
	if strings.EqualFold(resolved, selected) {
		return selected
	}
	session, err := config.ResolveCursorSessionBearer(ctx, b.cfg, b.provider)
	if err != nil {
		return selected
	}
	available, err := cursorauth.ListAvailableModels(ctx, session)
	if err != nil {
		return selected
	}
	for _, candidate := range []string{resolved, strings.TrimPrefix(resolved, "cursor-"), selected, strings.TrimPrefix(selected, "cursor-")} {
		for _, model := range available {
			if strings.EqualFold(candidate, model) {
				return model
			}
		}
	}
	return selected
}

func (b *cursorSubBackend) streamAgent(ctx context.Context, model, system string, messages []chatstore.Message, imageFiles map[int]string, defs []ToolDef, out io.Writer, effort string, opts StreamOpts) (cursorauth.ChatResult, error) {
	if b == nil || b.provider == nil {
		return cursorauth.ChatResult{}, fmt.Errorf("Cursor Sub backend missing provider")
	}
	tools, err := agentToolsFromDefs(defs)
	if err != nil {
		return cursorauth.ChatResult{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var session *cursorauth.AgentSession
	if b.pending != nil {
		if result, failed, ok := cursorToolResult(messages, b.pending.call.ID); ok {
			session = b.pending.session
			if err := session.ToolResult(b.pending.call, result, failed); err != nil {
				session.Close()
				b.pending = nil
				return cursorauth.ChatResult{}, err
			}
		} else {
			b.pending.session.Close()
		}
		b.pending = nil
	}
	if session == nil {
		key, err := b.cursorAgentKey(ctx)
		if err != nil {
			return cursorauth.ChatResult{}, err
		}
		fast := b.cfg != nil && b.cfg.EffectiveFastMode()
		model = b.selectAgentModel(ctx, model, effort, fast)
		user, media := latestCursorUser(messages, imageFiles)
		if user == "" && len(media) == 0 {
			return cursorauth.ChatResult{}, fmt.Errorf("Cursor Agent: no user message")
		}
		if user == "" {
			user = "Analyze the attached image."
		}
		history, err := agentHistory(messages, imageFiles)
		if err != nil {
			return cursorauth.ChatResult{}, err
		}
		session, err = cursorauth.OpenAgentSession(ctx, key, model, system, user, tools, history, media)
		if err != nil && strings.Contains(err.Error(), "token exchange") && (strings.Contains(err.Error(), "status 401") || strings.Contains(err.Error(), "status 403")) {
			key, err = b.renewCursorAgentKey(ctx)
			if err != nil {
				return cursorauth.ChatResult{}, err
			}
			session, err = cursorauth.OpenAgentSession(ctx, key, model, system, user, tools, history, media)
		}
		if err != nil {
			return cursorauth.ChatResult{}, err
		}
	}
	result := cursorauth.ChatResult{FinishReason: FinishReasonStop}
	for {
		event, ok := session.Next(ctx)
		if !ok {
			session.Close()
			return cursorauth.ChatResult{}, fmt.Errorf("Cursor Agent stream ended before turn completion")
		}
		if event.Err != nil {
			session.Close()
			return cursorauth.ChatResult{}, event.Err
		}
		if strings.HasPrefix(event.Unknown, "exec:") || strings.Contains(event.Unknown, "InteractionQuery") {
			session.Close()
			return cursorauth.ChatResult{}, fmt.Errorf("Cursor Agent requested unsupported operation %s", event.Unknown)
		}
		if event.Context != nil {
			if err := session.ContextResult(event.Context.ID, event.Context.ExecID, system, tools); err != nil {
				session.Close()
				return cursorauth.ChatResult{}, err
			}
		}
		if event.MCPState != nil {
			if err := session.MCPStateResult(*event.MCPState, tools); err != nil {
				session.Close()
				return cursorauth.ChatResult{}, err
			}
		}
		if event.Native != nil {
			if err := session.RejectNative(*event.Native); err != nil {
				session.Close()
				return cursorauth.ChatResult{}, err
			}
		}
		if event.Query != nil {
			if err := session.RejectQuery(*event.Query); err != nil {
				session.Close()
				return cursorauth.ChatResult{}, err
			}
		}
		if event.KV != nil {
			if err := session.KVResult(*event.KV); err != nil {
				session.Close()
				return cursorauth.ChatResult{}, err
			}
		}
		if event.Ended && result.Content == "" {
			if text, ok := session.LatestAssistantText(); ok {
				event.Text = text
			}
		}
		if event.Text != "" {
			result.Content += event.Text
			if opts.OnDelta != nil {
				opts.OnDelta("content", event.Text)
			}
			if out != nil {
				stopped, err := streamio.WriteContentLegacy(out, event.Text)
				if err != nil {
					session.Close()
					return cursorauth.ChatResult{}, err
				}
				if stopped {
					result.Content = streamio.TruncatedContent(out, result.Content)
					session.Close()
					return result, nil
				}
			}
		}
		if event.Thinking != "" {
			if opts.OnDelta != nil {
				opts.OnDelta("reasoning", event.Thinking)
			}
			if opts.ShowThinking && opts.ReasoningSink != nil {
				streamio.WriteReasoningDelta(opts.ReasoningSink, event.Thinking)
			}
		}
		if event.Tool != nil {
			allowed := false
			for _, tool := range tools {
				if tool.Name == event.Tool.Name {
					allowed = true
					break
				}
			}
			if !allowed {
				if err := session.ToolResult(*event.Tool, `{"error":"tool unavailable in Solomon"}`, true); err != nil {
					session.Close()
					return cursorauth.ChatResult{}, err
				}
				continue
			}
			b.pending = &cursorPendingTool{session: session, call: *event.Tool}
			result.ToolCalls = []cursorauth.ChatToolCall{{ID: event.Tool.ID, Name: event.Tool.Name, Arguments: event.Tool.Arguments}}
			result.FinishReason = FinishReasonToolCalls
			return result, nil
		}
		if event.Ended {
			result.Usage = event.Usage
			session.Close()
			return result, nil
		}
	}
}
