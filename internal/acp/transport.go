package acp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type params struct {
	ProtocolVersion int               `json:"protocolVersion"`
	CWD             string            `json:"cwd"`
	SessionID       string            `json:"sessionId"`
	MCPServers      []json.RawMessage `json:"mcpServers"`
	Prompt          []Content         `json:"prompt"`
	ModeID          string            `json:"modeId"`
	ModelID         string            `json:"modelId"`
	ConfigID        string            `json:"configId"`
	Value           string            `json:"value"`
	RequestID       json.RawMessage   `json:"requestId"`
	Cursor          string            `json:"cursor"`
}

func ValidateContent(blocks []Content) error {
	if len(blocks) == 0 {
		return fmt.Errorf("prompt is required")
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
		case "image":
			if b.Data == "" || !strings.HasPrefix(b.MimeType, "image/") {
				return fmt.Errorf("image data and image mimeType are required")
			}
		case "resource_link":
			if b.URI == "" {
				return fmt.Errorf("resource URI is required")
			}
		case "resource":
			if b.Resource == nil || b.Resource.URI == "" {
				return fmt.Errorf("embedded resource is required")
			}
		default:
			return fmt.Errorf("unsupported content type: %s", b.Type)
		}
	}
	return nil
}
func (s *Server) permission(ctx context.Context, id string, tool Update) error {
	encoded, _ := json.Marshal(map[string]any{"title": tool["title"], "rawInput": tool["rawInput"]})
	hash := sha256.Sum256(encoded)
	key := hex.EncodeToString(hash[:])
	s.mu.Lock()
	if v := s.sessions[id]; v != nil {
		if allowed, ok := v.record.Permissions[key]; ok {
			s.mu.Unlock()
			if allowed {
				return nil
			}
			return fmt.Errorf("tool execution denied by remembered session decision")
		}
	}
	s.nextID++
	rid := fmt.Sprintf("solomon-%d", s.nextID)
	raw, _ := json.Marshal(rid)
	ch := make(chan request, 1)
	s.pending[string(raw)] = ch
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, string(raw)); s.mu.Unlock() }()
	s.send(map[string]any{"jsonrpc": "2.0", "id": rid, "method": "session/request_permission", "params": map[string]any{"sessionId": id, "toolCall": tool, "options": []any{map[string]any{"optionId": "allow_once", "name": "Allow once", "kind": "allow_once"}, map[string]any{"optionId": "reject_once", "name": "Reject", "kind": "reject_once"}, map[string]any{"optionId": "allow_always", "name": "Allow this exact tool input for this session", "kind": "allow_always"}, map[string]any{"optionId": "reject_always", "name": "Reject this exact tool input for this session", "kind": "reject_always"}}}})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case response := <-ch:
		if len(response.Error) > 0 {
			return fmt.Errorf("client rejected permission request")
		}
		var result struct {
			Outcome struct {
				Outcome  string `json:"outcome"`
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		}
		if json.Unmarshal(response.Result, &result) != nil {
			return fmt.Errorf("invalid permission response")
		}
		if result.Outcome.Outcome == "selected" && (result.Outcome.OptionID == "allow_always" || result.Outcome.OptionID == "reject_always") {
			allowed := result.Outcome.OptionID == "allow_always"
			s.mu.Lock()
			if v := s.sessions[id]; v != nil {
				if v.record.Permissions == nil {
					v.record.Permissions = map[string]bool{}
				}
				v.record.Permissions[key] = allowed
			}
			s.mu.Unlock()
			if allowed {
				return nil
			}
		}
		if result.Outcome.Outcome == "selected" && result.Outcome.OptionID == "allow_once" {
			return nil
		}
		return fmt.Errorf("tool execution denied by client")
	}
}
