package mcp

import (
	"context"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
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

func (m *Manager) refreshTools(ctx context.Context, ss *serverSession) error {
	_, err := withSessionCall(m, ctx, ss, "tools/list", func(session *sdkmcp.ClientSession) (struct{}, error) {
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		if !supportsFeatureSession(session, "tools") {
			m.replaceServerTools(ss, nil)
			return struct{}{}, nil
		}
		var all []*sdkmcp.Tool
		cursor := ""
		for {
			listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
			res, err := session.ListTools(listCtx, &sdkmcp.ListToolsParams{Cursor: cursor})
			cancel()
			if err != nil {
				return struct{}{}, err
			}
			all = append(all, res.Tools...)
			if strings.TrimSpace(res.NextCursor) == "" {
				break
			}
			cursor = res.NextCursor
		}
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		tools := make([]RemoteTool, 0, len(all))
		for _, tool := range all {
			if tool == nil || !ss.cfg.ToolAllowed(tool.Name) {
				if tool != nil {
					logging.Log(logging.INFO_LOG_LEVEL, "MCP tool filtered", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "tool": tool.Name}})
				}
				continue
			}
			rt, err := AdaptTool(ss.cfg.Name, tool)
			if err != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "MCP tool schema skipped", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "tool": tool.Name, "err": err.Error()}})
				continue
			}
			tools = append(tools, rt)
		}
		m.replaceServerTools(ss, tools)
		return struct{}{}, nil
	})
	return err
}

func (m *Manager) refreshResources(ctx context.Context, ss *serverSession) error {
	_, err := withSessionCall(m, ctx, ss, "resources/list", func(session *sdkmcp.ClientSession) (struct{}, error) {
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		if !supportsFeatureSession(session, "resources") {
			m.replaceServerResources(ss, nil)
			return struct{}{}, nil
		}
		var resources []*sdkmcp.Resource
		cursor := ""
		for {
			listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
			res, err := session.ListResources(listCtx, &sdkmcp.ListResourcesParams{Cursor: cursor})
			cancel()
			if err != nil {
				return struct{}{}, err
			}
			resources = append(resources, res.Resources...)
			if strings.TrimSpace(res.NextCursor) == "" {
				break
			}
			cursor = res.NextCursor
		}
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		out := make([]RemoteResource, 0, len(resources))
		for _, resource := range resources {
			if resource != nil {
				out = append(out, RemoteResource{ServerName: ss.cfg.Name, Definition: resource})
			}
		}
		m.replaceServerResources(ss, out)
		return struct{}{}, nil
	})
	return err
}

func (m *Manager) refreshResourceTemplates(ctx context.Context, ss *serverSession) error {
	_, err := withSessionCall(m, ctx, ss, "resources/templates/list", func(session *sdkmcp.ClientSession) (struct{}, error) {
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		if !supportsFeatureSession(session, "resources") {
			m.replaceServerResourceTemplates(ss, nil)
			return struct{}{}, nil
		}
		var templates []*sdkmcp.ResourceTemplate
		cursor := ""
		for {
			listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
			res, err := session.ListResourceTemplates(listCtx, &sdkmcp.ListResourceTemplatesParams{Cursor: cursor})
			cancel()
			if err != nil {
				return struct{}{}, err
			}
			templates = append(templates, res.ResourceTemplates...)
			if strings.TrimSpace(res.NextCursor) == "" {
				break
			}
			cursor = res.NextCursor
		}
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		out := make([]RemoteResourceTemplate, 0, len(templates))
		for _, template := range templates {
			if template != nil {
				out = append(out, RemoteResourceTemplate{ServerName: ss.cfg.Name, Definition: template})
			}
		}
		m.replaceServerResourceTemplates(ss, out)
		return struct{}{}, nil
	})
	return err
}

func (m *Manager) refreshPrompts(ctx context.Context, ss *serverSession) error {
	_, err := withSessionCall(m, ctx, ss, "prompts/list", func(session *sdkmcp.ClientSession) (struct{}, error) {
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		if !supportsFeatureSession(session, "prompts") {
			m.replaceServerPrompts(ss, nil)
			return struct{}{}, nil
		}
		var prompts []*sdkmcp.Prompt
		cursor := ""
		for {
			listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
			res, err := session.ListPrompts(listCtx, &sdkmcp.ListPromptsParams{Cursor: cursor})
			cancel()
			if err != nil {
				return struct{}{}, err
			}
			prompts = append(prompts, res.Prompts...)
			if strings.TrimSpace(res.NextCursor) == "" {
				break
			}
			cursor = res.NextCursor
		}
		if !ss.ownsSession(session) {
			return struct{}{}, sdkmcp.ErrConnectionClosed
		}
		out := make([]RemotePrompt, 0, len(prompts))
		for _, prompt := range prompts {
			if prompt != nil {
				out = append(out, RemotePrompt{ServerName: ss.cfg.Name, Definition: prompt})
			}
		}
		m.replaceServerPrompts(ss, out)
		return struct{}{}, nil
	})
	return err
}

func (m *Manager) replaceServerTools(ss *serverSession, tools []RemoteTool) {
	if m == nil || ss == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, binding := range m.registry {
		if binding.server == ss {
			delete(m.registry, name)
		}
	}
	combined := make([]RemoteTool, 0, len(m.tools)+len(tools))
	for _, existing := range m.tools {
		if existing.ServerName != ss.cfg.Name {
			combined = append(combined, existing)
		}
	}
	used := make(map[string]bool, len(combined))
	for _, existing := range combined {
		used[existing.OpenAIName] = true
	}
	for _, tool := range tools {
		tool.OpenAIName = UniqueToolName(ExposedToolName(tool.ServerName, tool.ToolName), used)
		combined = append(combined, tool)
		m.registry[tool.OpenAIName] = &remoteBinding{server: ss, tool: tool}
	}
	m.tools = combined
	ss.tools = append([]RemoteTool(nil), tools...)
}

func (m *Manager) replaceServerResources(ss *serverSession, resources []RemoteResource) {
	if m == nil || ss == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	combined := make([]RemoteResource, 0, len(m.resources)+len(resources))
	for _, existing := range m.resources {
		if existing.ServerName != ss.cfg.Name {
			combined = append(combined, existing)
		}
	}
	m.resources = append(combined, resources...)
	ss.resources = append([]RemoteResource(nil), resources...)
}

func (m *Manager) replaceServerResourceTemplates(ss *serverSession, templates []RemoteResourceTemplate) {
	if m == nil || ss == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	combined := make([]RemoteResourceTemplate, 0, len(m.templates)+len(templates))
	for _, existing := range m.templates {
		if existing.ServerName != ss.cfg.Name {
			combined = append(combined, existing)
		}
	}
	m.templates = append(combined, templates...)
	ss.templates = append([]RemoteResourceTemplate(nil), templates...)
}

func (m *Manager) replaceServerPrompts(ss *serverSession, prompts []RemotePrompt) {
	if m == nil || ss == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	combined := make([]RemotePrompt, 0, len(m.prompts)+len(prompts))
	for _, existing := range m.prompts {
		if existing.ServerName != ss.cfg.Name {
			combined = append(combined, existing)
		}
	}
	m.prompts = append(combined, prompts...)
	ss.prompts = append([]RemotePrompt(nil), prompts...)
}
