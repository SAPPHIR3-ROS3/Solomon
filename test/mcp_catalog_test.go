package test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPStdioStartMarker is also used as a helper process by
// TestLegacyCloakMCPServerIsNotStarted. The native Cloak adapter must prevent
// an old community MCP command from ever being launched.
func TestMCPStdioStartMarker(t *testing.T) {
	if os.Getenv("SOLOMON_MCP_MARKER") != "1" {
		return
	}
	marker := os.Getenv("SOLOMON_MCP_MARKER_PATH")
	if marker == "" {
		t.Fatal("marker path is missing")
	}
	if err := os.WriteFile(marker, []byte("started"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyCloakMCPServerIsNotStarted(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	mgr := mcp.NewManager(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name:     "cloak-browser",
		Type:     mcp.TransportStdio,
		Command:  os.Args[0],
		Args:     []string{"-test.run=TestMCPStdioStartMarker"},
		Env:      map[string]string{"SOLOMON_MCP_MARKER": "1", "SOLOMON_MCP_MARKER_PATH": marker},
		Internal: true,
		Adapter:  "cloak",
	}}}, io.Discard)
	t.Cleanup(func() { _ = mgr.Close() })
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy Cloak MCP command was started; stat error=%v", err)
	}
}

func newCatalogMCPServer(t *testing.T, toolName string) *httptest.Server {
	t.Helper()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: toolName + "-server", Version: "1"}, nil)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: toolName, Description: toolName}, func(context.Context, *sdkmcp.CallToolRequest, map[string]any) (*sdkmcp.CallToolResult, struct{}, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}}}, struct{}{}, nil
	})
	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{Stateless: true})
	return httptest.NewServer(handler)
}

func TestInternalMCPToolsStayOutOfModelCatalog(t *testing.T) {
	publicHTTP := newCatalogMCPServer(t, "public_tool")
	t.Cleanup(publicHTTP.Close)
	internalHTTP := newCatalogMCPServer(t, "internal_tool")
	t.Cleanup(internalHTTP.Close)

	mgr := mcp.NewManager(context.Background(), &mcp.Config{Servers: []mcp.ServerConfig{
		{Name: "public", Type: mcp.TransportStreamableHTTP, URL: publicHTTP.URL},
		{Name: "exa-web", Type: mcp.TransportStreamableHTTP, URL: internalHTTP.URL, Internal: true, Adapter: "exa"},
	}}, io.Discard)
	t.Cleanup(func() { _ = mgr.Close() })

	publicName := "MCP.public.public_tool"
	internalName := "MCP.exa_web.internal_tool"
	entries := mgr.Catalog()
	if len(entries) != 1 || entries[0].Server != "public" || entries[0].Name != publicName {
		t.Fatalf("catalog=%#v", entries)
	}
	if len(mgr.OpenAITools()) != 1 || !mgr.HasTool(publicName) || mgr.HasTool(internalName) {
		t.Fatalf("public/internal tool visibility mismatch")
	}
	dump := mgr.ToolDump()
	if strings.Contains(dump, internalName) || !strings.Contains(dump, publicName) {
		t.Fatalf("tool dump=%s", dump)
	}
	adapters := mgr.InternalAdapters()
	if len(adapters) != 1 || adapters[0].ServerName != "exa-web" || adapters[0].Adapter != "exa" {
		t.Fatalf("internal adapters=%#v", adapters)
	}
}
