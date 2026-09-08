package mcp

import sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

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
			Schema:      mcpArgumentsSchema(t.Schema),
		})
	}
	return out
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
