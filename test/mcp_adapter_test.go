package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExposedToolNameSanitizesAndFallsBack(t *testing.T) {
	if got, want := mcp.ExposedToolName("file system", "read.file"), "MCP.file_system.read_file"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := mcp.ExposedToolName("", ""), "MCP.server.tool"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUniqueToolName(t *testing.T) {
	used := map[string]bool{}
	if got := mcp.UniqueToolName("MCPs-t", used); got != "MCPs-t" {
		t.Fatalf("got %q", got)
	}
	if got := mcp.UniqueToolName("MCPs-t", used); got != "MCPs-t-2" {
		t.Fatalf("got %q", got)
	}
}

func TestAdaptToolAcceptsObjectSchema(t *testing.T) {
	rt, err := mcp.AdaptTool("server", &sdkmcp.Tool{
		Name:        "search",
		Description: "Search things",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rt.ToolName != "search" || rt.Schema["type"] != "object" || rt.Schema["additionalProperties"] != false {
		t.Fatalf("%+v", rt)
	}
}

func TestAdaptToolRejectsInvalidSchema(t *testing.T) {
	for _, schema := range []any{nil, map[string]any{"type": "string"}, []any{}} {
		if _, err := mcp.AdaptTool("server", &sdkmcp.Tool{Name: "bad", InputSchema: schema}); err == nil {
			t.Fatalf("want error for %#v", schema)
		}
	}
}

func TestManagerToolDumpIncludesRemoteTools(t *testing.T) {
	mgr := mcp.NewManagerWithRemoteTools([]mcp.RemoteTool{{
		OpenAIName:  "MCP.server.search",
		Description: "Search remote data",
		Schema:      map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
	}})
	dump := mgr.ToolDump()
	if !strings.Contains(dump, "name: MCP.server.search") || !strings.Contains(dump, "Search remote data") || !strings.Contains(dump, `"q"`) {
		t.Fatalf("unexpected dump: %s", dump)
	}
}
