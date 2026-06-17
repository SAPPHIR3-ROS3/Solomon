package mcp

type CatalogEntry struct {
	Name        string         `json:"name"`
	Server      string         `json:"server"`
	Tool        string         `json:"tool"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema,omitempty"`
}

func (m *Manager) Catalog() []CatalogEntry {
	if m == nil || len(m.tools) == 0 {
		return nil
	}
	out := make([]CatalogEntry, 0, len(m.tools))
	for _, t := range m.tools {
		out = append(out, CatalogEntry{
			Name:        t.OpenAIName,
			Server:      t.ServerName,
			Tool:        t.ToolName,
			Description: t.Description,
			Schema:      t.Schema,
		})
	}
	return out
}
