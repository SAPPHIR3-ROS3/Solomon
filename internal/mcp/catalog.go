package mcp

import (
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type CatalogEntry struct {
	Name        string         `json:"name"`
	Server      string         `json:"server"`
	Tool        string         `json:"tool"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// InternalAdapterInfo is the non-sensitive part of an internal MCP server
// configuration needed by a host adapter factory.
type InternalAdapterInfo struct {
	ServerName string
	Adapter    string
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

// InternalAdapters returns configured host-managed adapters, including ones
// that are not currently connected. The latter still need to be represented so
// the router can report a backend failure and try its fallback.
func (m *Manager) InternalAdapters() []InternalAdapterInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg == nil {
		return nil
	}
	out := make([]InternalAdapterInfo, 0)
	for _, server := range m.cfg.Servers {
		if !server.Internal || strings.TrimSpace(server.Adapter) == "" {
			continue
		}
		out = append(out, InternalAdapterInfo{ServerName: server.Name, Adapter: server.Adapter})
	}
	return out
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
		if m.serverInternalLocked(t.ServerName) {
			continue
		}
		out = append(out, CatalogEntry{
			Name:        t.OpenAIName,
			Server:      t.ServerName,
			Tool:        t.ToolName,
			Description: t.Description,
			Schema:      mcpArgumentsSchema(t.Schema),
		})
	}
	return out
}

// serverInternalLocked reports whether a configured server is host-managed
// rather than model-facing. Callers must hold m.mu.RLock or m.mu.Lock.
func (m *Manager) serverInternalLocked(name string) bool {
	name = strings.TrimSpace(name)
	for _, server := range m.servers {
		if server != nil && server.cfg.Name == name {
			return server.cfg.Internal
		}
	}
	if m.cfg != nil {
		for _, server := range m.cfg.Servers {
			if server.Name == name {
				return server.Internal
			}
		}
	}
	return false
}

// mcpArgumentsSchema returns the schema that the remote server expects. The
// manager stores Solomon's required intent field alongside tool schemas for
// native dispatch, but Code Mode passes intent as its first SDK argument and
// must not present it as an MCP argument again.
func mcpArgumentsSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	out := make(map[string]any, len(schema))
	for key, value := range schema {
		out[key] = value
	}
	if existing, ok := schema["properties"].(map[string]any); ok {
		properties := make(map[string]any, len(existing))
		for key, value := range existing {
			if key != "intent" {
				properties[key] = value
			}
		}
		out["properties"] = properties
	}
	switch required := schema["required"].(type) {
	case []any:
		filtered := make([]any, 0, len(required))
		for _, value := range required {
			if name, ok := value.(string); !ok || name != "intent" {
				filtered = append(filtered, value)
			}
		}
		if len(filtered) == 0 {
			delete(out, "required")
		} else {
			out["required"] = filtered
		}
	case []string:
		filtered := make([]string, 0, len(required))
		for _, name := range required {
			if name != "intent" {
				filtered = append(filtered, name)
			}
		}
		if len(filtered) == 0 {
			delete(out, "required")
		} else {
			out["required"] = filtered
		}
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
