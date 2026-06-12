package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

type RemoteTool struct {
	OpenAIName  string
	ServerName  string
	ToolName    string
	Description string
	Schema      map[string]any
}

func OpenAITool(t RemoteTool) openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.OpenAIName,
				Description: openai.String(t.Description),
				Parameters:  openai.FunctionParameters(t.Schema),
			},
		},
	}
}

func AdaptTool(serverName string, tool *sdkmcp.Tool) (RemoteTool, error) {
	if tool == nil {
		return RemoteTool{}, fmt.Errorf("nil MCP tool")
	}
	if strings.TrimSpace(tool.Name) == "" {
		return RemoteTool{}, fmt.Errorf("MCP tool has empty name")
	}
	schema, err := adaptInputSchema(tool.InputSchema)
	if err != nil {
		return RemoteTool{}, err
	}
	desc := strings.TrimSpace(tool.Description)
	if desc == "" {
		desc = "Remote MCP tool"
	}
	desc = fmt.Sprintf("%s (from MCP server %s)", desc, serverName)
	return RemoteTool{
		ServerName:  serverName,
		ToolName:    tool.Name,
		Description: desc,
		Schema:      schema,
	}, nil
}

func adaptInputSchema(input any) (map[string]any, error) {
	if input == nil {
		return nil, fmt.Errorf("missing inputSchema")
	}
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal inputSchema: %w", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, fmt.Errorf("inputSchema must be a JSON object: %w", err)
	}
	if typ, _ := schema["type"].(string); typ != "object" {
		return nil, fmt.Errorf("inputSchema type must be object")
	}
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = map[string]any{}
	}
	if _, ok := schema["additionalProperties"]; !ok {
		schema["additionalProperties"] = false
	}
	if req, ok := schema["required"]; !ok || req == nil {
		schema["required"] = []any{}
	}
	return schema, nil
}

var toolNamePart = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func ExposedToolName(serverName, toolName string) string {
	server := sanitizeToolPart(serverName, "server")
	tool := sanitizeToolPart(toolName, "tool")
	return "MCP" + server + "-" + tool
}

func sanitizeToolPart(s, fallback string) string {
	s = strings.TrimSpace(s)
	s = toolNamePart.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_-")
	if s == "" {
		return fallback
	}
	return s
}

func UniqueToolName(base string, used map[string]bool) string {
	if !used[base] {
		used[base] = true
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if !used[name] {
			used[name] = true
			return name
		}
	}
}
