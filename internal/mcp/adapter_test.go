package mcp

import (
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExposedToolNameSanitizesAndFallsBack(t *testing.T) {
	if got, want := ExposedToolName("file system", "read.file"), "MCPfile_system-read_file"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := ExposedToolName("", ""), "MCPserver-tool"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUniqueToolName(t *testing.T) {
	used := map[string]bool{}
	if got := UniqueToolName("MCPs-t", used); got != "MCPs-t" {
		t.Fatalf("got %q", got)
	}
	if got := UniqueToolName("MCPs-t", used); got != "MCPs-t-2" {
		t.Fatalf("got %q", got)
	}
}

func TestAdaptToolAcceptsObjectSchema(t *testing.T) {
	rt, err := AdaptTool("server", &sdkmcp.Tool{
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
		if _, err := AdaptTool("server", &sdkmcp.Tool{Name: "bad", InputSchema: schema}); err == nil {
			t.Fatalf("want error for %#v", schema)
		}
	}
}

func TestManagerToolDumpIncludesRemoteTools(t *testing.T) {
	mgr := &Manager{tools: []RemoteTool{{
		OpenAIName:  "MCPserver-search",
		Description: "Search remote data",
		Schema:      map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
	}}}
	dump := mgr.ToolDump()
	if !strings.Contains(dump, "name: MCPserver-search") || !strings.Contains(dump, "Search remote data") || !strings.Contains(dump, `"q"`) {
		t.Fatalf("unexpected dump: %s", dump)
	}
}
