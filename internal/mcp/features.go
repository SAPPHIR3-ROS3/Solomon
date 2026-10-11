package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerInfo is a snapshot of a connected MCP server's negotiated session.
type ServerInfo struct {
	Name            string
	Type            string
	URL             string
	ProtocolVersion string
	Implementation  *sdkmcp.Implementation
	Capabilities    *sdkmcp.ServerCapabilities
	Instructions    string
}

func (m *Manager) clientOptions(ss *serverSession) sdkmcp.ClientOptions {
	opts := m.options.ClientOptions
	userToolChanged := opts.ToolListChangedHandler
	userPromptChanged := opts.PromptListChangedHandler
	userResourceChanged := opts.ResourceListChangedHandler
	userResourceUpdated := opts.ResourceUpdatedHandler

	// Installing these handlers does two things: it keeps the Manager's
	// snapshots correct and makes the Go SDK opt into July's
	// subscriptions/listen stream for modern servers.
	opts.ToolListChangedHandler = func(ctx context.Context, req *sdkmcp.ToolListChangedRequest) {
		if userToolChanged != nil {
			userToolChanged(ctx, req)
		}
		if ss != nil {
			m.refreshCatalogAsync(ctx, ss, "tools", m.refreshTools)
		}
	}
	opts.PromptListChangedHandler = func(ctx context.Context, req *sdkmcp.PromptListChangedRequest) {
		if userPromptChanged != nil {
			userPromptChanged(ctx, req)
		}
		if ss != nil {
			m.refreshCatalogAsync(ctx, ss, "prompts", m.refreshPrompts)
		}
	}
	opts.ResourceListChangedHandler = func(ctx context.Context, req *sdkmcp.ResourceListChangedRequest) {
		if userResourceChanged != nil {
			userResourceChanged(ctx, req)
		}
		if ss != nil {
			refreshCtx := context.WithoutCancel(ctx)
			go func(server *serverSession) {
				if err := m.refreshResources(refreshCtx, server); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "MCP resources catalog refresh failed", logging.LogOptions{Params: map[string]any{"server": server.cfg.Name, "err": err.Error()}})
				}
				if err := m.refreshResourceTemplates(refreshCtx, server); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "MCP resource templates refresh failed", logging.LogOptions{Params: map[string]any{"server": server.cfg.Name, "err": err.Error()}})
				}
			}(ss)
		}
	}
	opts.ResourceUpdatedHandler = func(ctx context.Context, req *sdkmcp.ResourceUpdatedNotificationRequest) {
		if userResourceUpdated != nil {
			userResourceUpdated(ctx, req)
		}
	}
	if m.options.DisableCatalogSubscriptions {
		opts.ToolListChangedHandler = nil
		opts.PromptListChangedHandler = nil
		opts.ResourceListChangedHandler = nil
	}
	return opts
}

// subscriptionClientOptions creates options for a dedicated modern resource
// subscription session. The main session owns the July subscriptions/listen
// stream for catalog changes; resource subscriptions must not share that
// session because the SDK cancels each resource listener independently.
func (m *Manager) subscriptionClientOptions(ss *serverSession) sdkmcp.ClientOptions {
	opts := m.clientOptions(ss)
	opts.ToolListChangedHandler = nil
	opts.PromptListChangedHandler = nil
	opts.ResourceListChangedHandler = nil
	return opts
}

func (m *Manager) refreshCatalogAsync(ctx context.Context, ss *serverSession, feature string, refresh func(context.Context, *serverSession) error) {
	ctx = context.WithoutCancel(ctx)
	go func() {
		if err := refresh(ctx, ss); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP catalog refresh failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "feature": feature, "err": err.Error()}})
		}
	}()
}

func (m *Manager) registerServerCatalog(ctx context.Context, ss *serverSession) error {
	// A server may legitimately expose only resources, prompts, or completions.
	// Never make tools/list a prerequisite for establishing the session.
	if supportsFeature(ss, "tools") {
		if err := m.refreshTools(ctx, ss); err != nil {
			if m.closed.Load() || ctx.Err() != nil {
				return err
			}
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP server tools/list failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
		}
	}
	if supportsFeature(ss, "resources") {
		if err := m.refreshResources(ctx, ss); err != nil {
			if m.closed.Load() || ctx.Err() != nil {
				return err
			}
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP server resources/list failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
		}
		if err := m.refreshResourceTemplates(ctx, ss); err != nil {
			if m.closed.Load() || ctx.Err() != nil {
				return err
			}
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP server resources/templates/list failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
		}
	}
	if supportsFeature(ss, "prompts") {
		if err := m.refreshPrompts(ctx, ss); err != nil {
			if m.closed.Load() || ctx.Err() != nil {
				return err
			}
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP server prompts/list failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
		}
	}
	return nil
}

func supportsFeature(ss *serverSession, feature string) bool {
	if ss == nil {
		return false
	}
	return supportsFeatureSession(ss.currentSession(), feature)
}

func supportsFeatureSession(session *sdkmcp.ClientSession, feature string) bool {
	if session == nil {
		return false
	}
	result := session.InitializeResult()
	if result == nil || result.Capabilities == nil {
		return true
	}
	switch feature {
	case "tools":
		return result.Capabilities.Tools != nil
	case "resources":
		return result.Capabilities.Resources != nil
	case "prompts":
		return result.Capabilities.Prompts != nil
	case "completions":
		return result.Capabilities.Completions != nil
	default:
		return true
	}
}

func (m *Manager) sessionFor(ctx context.Context, serverName string) (*serverSession, error) {
	if m == nil {
		return nil, fmt.Errorf("MCP manager unavailable")
	}
	if err := m.WaitReady(ctx); err != nil {
		return nil, err
	}
	serverName = strings.TrimSpace(serverName)
	m.mu.RLock()
	var found *serverSession
	for _, ss := range m.servers {
		if ss.cfg.Name == serverName {
			found = ss
			break
		}
	}
	m.mu.RUnlock()
	if found == nil {
		return nil, fmt.Errorf("unknown MCP server %q", serverName)
	}
	if _, err := m.ensureSession(ctx, found); err != nil {
		return nil, unavailableError(serverName, "session", err)
	}
	return found, nil
}

func (m *Manager) AddRoots(roots ...*sdkmcp.Root) {
	if m == nil || len(roots) == 0 {
		return
	}
	m.mu.Lock()
	updated := make([]*sdkmcp.Root, 0, len(m.options.Roots)+len(roots))
	positions := make(map[string]int, len(m.options.Roots)+len(roots))
	for _, root := range m.options.Roots {
		if root != nil {
			if _, ok := positions[root.URI]; !ok {
				positions[root.URI] = len(updated)
				updated = append(updated, root)
			}
		}
	}
	for _, root := range roots {
		if root != nil {
			if index, ok := positions[root.URI]; ok {
				updated[index] = root
			} else {
				positions[root.URI] = len(updated)
				updated = append(updated, root)
			}
		}
	}
	m.options.Roots = updated
	servers := append([]*serverSession(nil), m.servers...)
	m.mu.Unlock()
	for _, ss := range servers {
		if client := ss.currentClient(); client != nil {
			client.AddRoots(roots...)
		}
		for _, client := range ss.subscriptionClients() {
			client.AddRoots(roots...)
		}
	}
}

func (m *Manager) RemoveRoots(uris ...string) {
	if m == nil || len(uris) == 0 {
		return
	}
	m.mu.Lock()
	remaining := make([]*sdkmcp.Root, 0, len(m.options.Roots))
	remove := make(map[string]struct{}, len(uris))
	for _, uri := range uris {
		remove[uri] = struct{}{}
	}
	for _, root := range m.options.Roots {
		if root != nil {
			if _, ok := remove[root.URI]; !ok {
				remaining = append(remaining, root)
			}
		}
	}
	m.options.Roots = remaining
	servers := append([]*serverSession(nil), m.servers...)
	m.mu.Unlock()
	for _, ss := range servers {
		if client := ss.currentClient(); client != nil {
			client.RemoveRoots(uris...)
		}
		for _, client := range ss.subscriptionClients() {
			client.RemoveRoots(uris...)
		}
	}
}

func (m *Manager) Servers() []ServerInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerInfo, 0, len(m.servers))
	for _, ss := range m.servers {
		result := ss.sessionInfo()
		info := ServerInfo{Name: ss.cfg.Name, Type: ss.cfg.Type, URL: ss.cfg.URL}
		if result != nil {
			info.ProtocolVersion = result.ProtocolVersion
			info.Implementation = result.ServerInfo
			info.Capabilities = result.Capabilities
			info.Instructions = result.Instructions
		}
		out = append(out, info)
	}
	return out
}

// ListResources refreshes and returns resources for one connected server.
func (m *Manager) ListResources(ctx context.Context, serverName string) ([]*sdkmcp.Resource, error) {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return nil, err
	}
	if err := m.refreshResources(ctx, ss); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*sdkmcp.Resource, 0, len(ss.resources))
	for _, resource := range ss.resources {
		out = append(out, resource.Definition)
	}
	return out, nil
}

// ListResourceTemplates refreshes and returns resource URI templates for one
// connected server.
func (m *Manager) ListResourceTemplates(ctx context.Context, serverName string) ([]*sdkmcp.ResourceTemplate, error) {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return nil, err
	}
	if err := m.refreshResourceTemplates(ctx, ss); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*sdkmcp.ResourceTemplate, 0, len(ss.templates))
	for _, template := range ss.templates {
		out = append(out, template.Definition)
	}
	return out, nil
}

// ReadResource reads a resource from one connected MCP server.
func (m *Manager) ReadResource(ctx context.Context, serverName, uri string) (*sdkmcp.ReadResourceResult, error) {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return nil, err
	}
	result, err := withSessionCall(m, ctx, ss, "resources/read", func(session *sdkmcp.ClientSession) (*sdkmcp.ReadResourceResult, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
		defer cancel()
		return session.ReadResource(callCtx, &sdkmcp.ReadResourceParams{URI: uri})
	})
	return result, err
}

// ListPrompts refreshes and returns prompts for one connected server.
func (m *Manager) ListPrompts(ctx context.Context, serverName string) ([]*sdkmcp.Prompt, error) {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return nil, err
	}
	if err := m.refreshPrompts(ctx, ss); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*sdkmcp.Prompt, 0, len(ss.prompts))
	for _, prompt := range ss.prompts {
		out = append(out, prompt.Definition)
	}
	return out, nil
}

// GetPrompt gets a rendered prompt from one connected MCP server.
func (m *Manager) GetPrompt(ctx context.Context, serverName, name string, arguments map[string]string) (*sdkmcp.GetPromptResult, error) {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return nil, err
	}
	result, err := withSessionCall(m, ctx, ss, "prompts/get", func(session *sdkmcp.ClientSession) (*sdkmcp.GetPromptResult, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
		defer cancel()
		return session.GetPrompt(callCtx, &sdkmcp.GetPromptParams{Name: name, Arguments: arguments})
	})
	return result, err
}

// Complete asks one MCP server for argument completions.
func (m *Manager) Complete(ctx context.Context, serverName string, params *sdkmcp.CompleteParams) (*sdkmcp.CompleteResult, error) {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return nil, err
	}
	if !supportsFeature(ss, "completions") {
		return nil, fmt.Errorf("MCP server %q does not advertise completions", serverName)
	}
	result, err := withSessionCall(m, ctx, ss, "completion/complete", func(session *sdkmcp.ClientSession) (*sdkmcp.CompleteResult, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
		defer cancel()
		return session.Complete(callCtx, params)
	})
	return result, err
}

// Ping checks the connection to one MCP server.
func (m *Manager) Ping(ctx context.Context, serverName string) error {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	if usesModernProtocol(ss) {
		result := ss.sessionInfo()
		version := "unknown"
		if result != nil {
			version = result.ProtocolVersion
		}
		return fmt.Errorf("MCP server %q does not support ping in protocol %s", serverName, version)
	}
	_, err = withSessionCall(m, ctx, ss, "ping", func(session *sdkmcp.ClientSession) (struct{}, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
		defer cancel()
		return struct{}{}, session.Ping(callCtx, nil)
	})
	return err
}

// SetLoggingLevel requests a minimum log level from one MCP server. Logging
// is deprecated by MCP July but remains part of the compatibility surface.
func (m *Manager) SetLoggingLevel(ctx context.Context, serverName string, level sdkmcp.LoggingLevel) error {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	if usesModernProtocol(ss) {
		result := ss.sessionInfo()
		version := "unknown"
		if result != nil {
			version = result.ProtocolVersion
		}
		return fmt.Errorf("MCP server %q does not support logging level in protocol %s", serverName, version)
	}
	_, err = withSessionCall(m, ctx, ss, "logging/setLevel", func(session *sdkmcp.ClientSession) (struct{}, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
		defer cancel()
		return struct{}{}, session.SetLoggingLevel(callCtx, &sdkmcp.SetLoggingLevelParams{Level: level})
	})
	return err
}

func usesModernProtocol(ss *serverSession) bool {
	if ss == nil {
		return false
	}
	result := ss.sessionInfo()
	return result != nil && result.ProtocolVersion >= "2026-07-28"
}
