package test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestAgentModeBlocksDirectMCP(t *testing.T) {
	mgr := solomonmcp.NewManagerWithRemoteTools([]solomonmcp.RemoteTool{{
		OpenAIName:  "MCP.github.create_issue",
		ServerName:  "github",
		ToolName:    "create_issue",
		Description: "Create issue",
		Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
	}})
	env := &tools.Env{MCP: mgr}
	_, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "MCP.github.create_issue",
		Args: json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("expected direct MCP tool to be blocked in agent mode")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}
