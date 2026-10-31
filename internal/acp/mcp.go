package acp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type NamedValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type MCPServer struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Command string       `json:"command"`
	Args    []string     `json:"args"`
	Env     []NamedValue `json:"env"`
	URL     string       `json:"url"`
	Headers []NamedValue `json:"headers"`
}

func ParseMCP(raw []json.RawMessage) ([]MCPServer, error) {
	result := make([]MCPServer, 0, len(raw))
	seen := map[string]bool{}
	for _, data := range raw {
		var server MCPServer
		if err := json.Unmarshal(data, &server); err != nil {
			return nil, err
		}
		if server.Name == "" || seen[server.Name] {
			return nil, fmt.Errorf("MCP server names must be nonempty and unique")
		}
		seen[server.Name] = true
		switch server.Type {
		case "", "stdio":
			if strings.TrimSpace(server.Command) == "" {
				return nil, fmt.Errorf("MCP command is required")
			}
			for _, e := range server.Env {
				if e.Name == "" || strings.ContainsAny(e.Name, "=\x00") || strings.ContainsRune(e.Value, 0) {
					return nil, fmt.Errorf("invalid MCP environment variable")
				}
			}
		case "http", "sse":
			u, err := url.Parse(server.URL)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, fmt.Errorf("MCP URL must use http or https")
			}
			for _, h := range server.Headers {
				if h.Name == "" || strings.ContainsAny(h.Name+h.Value, "\r\n") {
					return nil, fmt.Errorf("invalid MCP header")
				}
			}
		default:
			return nil, fmt.Errorf("unsupported MCP transport: %s", server.Type)
		}
		result = append(result, server)
	}
	return result, nil
}
