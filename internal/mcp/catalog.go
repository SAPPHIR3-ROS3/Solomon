package mcp

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type CatalogEntry struct {
	Name        string         `json:"name"`
	Server      string         `json:"server"`
	Tool        string         `json:"tool"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// RemoteResource is a resource advertised by a connected MCP server.
type RemoteResource struct {
	ServerName string
	Definition *sdkmcp.Resource
}

// RemoteResourceTemplate is a resource URI template advertised by a connected
// MCP server.
type RemoteResourceTemplate struct {
	ServerName string
	Definition *sdkmcp.ResourceTemplate
}

// RemotePrompt is a prompt advertised by a connected MCP server.
type RemotePrompt struct {
	ServerName string
	Definition *sdkmcp.Prompt
}

// Tools returns a snapshot of the complete MCP tool descriptors currently
// advertised by connected servers. The OpenAI projection is available through
// OpenAITools; this view retains MCP-specific metadata and output schemas.
func (m *Manager) Tools() []RemoteTool {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RemoteTool(nil), m.tools...)
}

func (m *Manager) Catalog() []CatalogEntry {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.tools) == 0 {
		return nil
	}
	out := make([]CatalogEntry, 0, len(m.tools))
	for _, t := range m.tools {
		out = append(out, CatalogEntry{
			Name:        t.OpenAIName,
			Server:      t.ServerName,
			Tool:        t.ToolName,
			Description: t.Description,
			Schema:      tooling.SchemaWithRequiredToolIntent(t.Schema),
		})
	}
	return out
}

// Resources returns a snapshot of all resources currently advertised by
// connected servers.
func (m *Manager) Resources() []RemoteResource {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RemoteResource(nil), m.resources...)
}

// ResourceTemplates returns a snapshot of all resource templates currently
// advertised by connected servers.
func (m *Manager) ResourceTemplates() []RemoteResourceTemplate {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RemoteResourceTemplate(nil), m.templates...)
}

// Prompts returns a snapshot of all prompts currently advertised by connected
// servers.
func (m *Manager) Prompts() []RemotePrompt {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RemotePrompt(nil), m.prompts...)
}
