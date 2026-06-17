package test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestSearchToolsListsNativeMCPTools(t *testing.T) {
	mgr := solomonmcp.NewManagerWithRemoteTools([]solomonmcp.RemoteTool{{
		OpenAIName:  "MCP.github.create_issue",
		ServerName:  "github",
		ToolName:    "create_issue",
		Description: "Create a GitHub issue",
		Schema:      map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}}},
	}})
	env := &tools.Env{MCP: mgr}
	raw, err := json.Marshal(map[string]string{"query": "mcp"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "searchTools", Args: raw})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("unexpected type %T", out)
	}
	list, ok := m["tools"].([]map[string]string)
	if !ok {
		t.Fatalf("tools: %#v", m["tools"])
	}
	found := false
	for _, e := range list {
		if e["name"] == "MCP.github.create_issue" && e["origin_mode"] == "native" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("MCP tool missing from searchTools: %#v", list)
	}
}