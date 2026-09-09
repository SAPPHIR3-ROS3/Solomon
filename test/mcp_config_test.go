package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
)

func TestParseConfigExpandsEnvAndFiltersTools(t *testing.T) {
	t.Setenv("WORKSPACE", "/tmp/workspace")
	t.Setenv("MCP_TOKEN", "secret")
	t.Setenv("MCP_CLIENT_ID", "client-id")
	raw := []byte(`{
		"mcpServers": {
			"filesystem": {
				"command": "npx",
				"args": ["-y", "$WORKSPACE", "Bearer $MCP_TOKEN"],
				"env": {"TOKEN": "$MCP_TOKEN"},
				"allow": ["read_file"],
				"deny": ["write_file"]
			},
			"remote": {
				"url": "https://example.com/$MCP_TOKEN/mcp",
				"headers": {"Authorization": "Bearer $MCP_TOKEN"},
				"oauth": {"clientId": "$MCP_CLIENT_ID", "clientSecret": "$MCP_TOKEN", "redirectUrl": "http://127.0.0.1/callback", "requestRefreshToken": true},
				"timeout": 300000,
				"allow": [],
				"deny": []
			}
		}
	}`)
	cfg, err := mcp.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("servers=%d", len(cfg.Servers))
	}
	fs := cfg.Servers[0]
	if fs.Name != "filesystem" || fs.Type != mcp.TransportStdio || fs.Args[1] != "/tmp/workspace" || fs.Env["TOKEN"] != "secret" {
		t.Fatalf("filesystem config: %+v", fs)
	}
	if !fs.ToolAllowed("read_file") || fs.ToolAllowed("write_file") || fs.ToolAllowed("other") {
		t.Fatalf("filter semantics failed")
	}
	remote := cfg.Servers[1]
	if remote.Type != mcp.TransportStreamableHTTP || !strings.Contains(remote.URL, "secret") || remote.Headers["Authorization"] != "Bearer secret" || remote.Timeout != 300000 {
		t.Fatalf("remote config: %+v", remote)
	}
	if remote.OAuth == nil || remote.OAuth.ClientID != "client-id" || remote.OAuth.ClientSecret != "secret" || !remote.OAuth.RequestRefresh {
		t.Fatalf("remote OAuth config: %+v", remote.OAuth)
	}
	if !remote.ToolAllowed("anything") {
		t.Fatalf("empty allow/deny should allow every tool")
	}
}

func TestParseConfigValidationErrors(t *testing.T) {
	tests := []string{
		`{`,
		`{"servers": {}}`,
		`{"mcpServers": []}`,
		`{"mcpServers": {"bad": {"type": "streamable-http"}}}`,
		`{"mcpServers": {"bad": {"type": "sse"}}}`,
		`{"mcpServers": {"bad": {"type": "stdio"}}}`,
		`{"mcpServers": {"bad": {"type": "unknown"}}}`,
		`{"mcpServers": {"bad": {"command": "x", "allow": "read"}}}`,
	}
	for _, raw := range tests {
		if _, err := mcp.ParseConfig([]byte(raw)); err == nil {
			t.Fatalf("want error for %s", raw)
		}
	}
}

func TestParseConfigSupportsLegacySSETransport(t *testing.T) {
	cfg, err := mcp.ParseConfig([]byte(`{"mcpServers":{"legacy":{"type":"sse","url":"https://example.com/sse"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 1 || cfg.Servers[0].Type != mcp.TransportSSE {
		t.Fatalf("legacy SSE config = %#v", cfg.Servers)
	}
}

func TestParseConfigSupportsInternalAdapters(t *testing.T) {
	cfg, err := mcp.ParseConfig([]byte(`{"mcpServers":{"exa-web":{"url":"https://mcp.exa.ai/mcp","internal":true,"adapter":"EXA","allow":["web_search_exa","web_fetch_exa"]},"parallel-web":{"url":"https://search.parallel.ai/mcp","internal":true,"adapter":"parallel","allow":["web_search","web_fetch"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 2 || !cfg.Servers[0].Internal || cfg.Servers[0].Adapter != "exa" || !cfg.Servers[0].ToolAllowed("web_search_exa") || !cfg.Servers[0].ToolAllowed("web_fetch_exa") || !cfg.Servers[1].ToolAllowed("web_search") || !cfg.Servers[1].ToolAllowed("web_fetch") {
		t.Fatalf("internal adapter config = %#v", cfg.Servers)
	}
}

func TestParseConfigRejectsPublicAdapter(t *testing.T) {
	_, err := mcp.ParseConfig([]byte(`{"mcpServers":{"exa-web":{"url":"https://mcp.exa.ai/mcp","adapter":"exa"}}}`))
	if err == nil || !strings.Contains(err.Error(), "internal") {
		t.Fatalf("got %v", err)
	}
}

func TestParseConfigMissingEnvVar(t *testing.T) {
	t.Setenv("PRESENT", "ok")
	_, err := mcp.ParseConfig([]byte(`{"mcpServers": {"x": {"command": "$PRESENT", "args": ["$MISSING"]}}}`))
	if err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "missing.json")
	t.Setenv("SOLOMON_MCP_CONFIG", p)
	cfg, err := mcp.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != p || len(cfg.Servers) != 0 {
		t.Fatalf("%+v", cfg)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("config file should not be created")
	}
}

func TestConfiguredServerCount(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mcp.json")
	t.Setenv("SOLOMON_MCP_CONFIG", p)
	if err := os.WriteFile(p, []byte(`{"mcpServers":{"a":{"command":"x"},"b":{"type":"streamable-http","url":"https://example.com/mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := mcp.ConfiguredServerCount()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("got %d want 2", n)
	}
}
