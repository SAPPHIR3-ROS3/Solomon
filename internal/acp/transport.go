package acp

import (
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
