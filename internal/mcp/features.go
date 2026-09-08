package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP server tools/list failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
		}
	}
	if supportsFeature(ss, "resources") {
		if err := m.refreshResources(ctx, ss); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP server resources/list failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
		}
		if err := m.refreshResourceTemplates(ctx, ss); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP server resources/templates/list failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
		}
	}
	if supportsFeature(ss, "prompts") {
		if err := m.refreshPrompts(ctx, ss); err != nil {
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

// Subscribe subscribes to updates for a resource. For MCP July servers the
// SDK maps this to subscriptions/listen; for legacy servers it uses the
// resources/subscribe request.
func (m *Manager) Subscribe(ctx context.Context, serverName, uri string) error {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return fmt.Errorf("MCP resource subscription requires a URI")
	}
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	ss.markSubscriptionDesired(uri)
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	if usesModernProtocol(ss) {
		if err := m.subscribeModern(callCtx, ss, uri); err != nil {
			if !isSessionFailure(err) {
				ss.unmarkSubscriptionDesired(uri)
			}
			return err
		}
		return nil
	}
	_, err = withSessionCall(m, callCtx, ss, "resources/subscribe", func(session *sdkmcp.ClientSession) (struct{}, error) {
		return struct{}{}, session.Subscribe(callCtx, &sdkmcp.SubscribeParams{URI: uri})
	})
	if err != nil && !errors.Is(err, ErrServerUnavailable) {
		ss.unmarkSubscriptionDesired(uri)
	}
	return err
}

// Unsubscribe stops a resource subscription.
func (m *Manager) Unsubscribe(ctx context.Context, serverName, uri string) error {
	uri = strings.TrimSpace(uri)
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	ss.unmarkSubscriptionDesired(uri)
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	if usesModernProtocol(ss) {
		return m.unsubscribeModern(callCtx, ss, uri)
	}
	_, err = withSessionCall(m, callCtx, ss, "resources/unsubscribe", func(session *sdkmcp.ClientSession) (struct{}, error) {
		return struct{}{}, session.Unsubscribe(callCtx, &sdkmcp.UnsubscribeParams{URI: uri})
	})
	return err
}

func (m *Manager) subscribeModern(ctx context.Context, ss *serverSession, uri string) error {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return fmt.Errorf("MCP resource subscription requires a URI")
	}
	if !ss.subscriptionDesired(uri) {
		return nil
	}
	ss.subscriptionsMu.Lock()
	if ss.subscriptions == nil {
		ss.subscriptions = make(map[string]*resourceSubscription)
	}
	if _, ok := ss.subscriptions[uri]; ok {
		ss.subscriptionsMu.Unlock()
		return nil
	}
	ss.subscriptionsMu.Unlock()

	clientOptions := m.subscriptionClientOptions(ss)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "solomon", Version: "dev", Title: "Solomon"}, &clientOptions)
	for _, root := range m.rootsSnapshot() {
		if root != nil {
			client.AddRoots(root)
		}
	}
	transport, err := m.transportFor(ctx, ss.cfg, m.stderr)
	if err != nil {
		return err
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return err
	}
	if err := session.Subscribe(ctx, &sdkmcp.SubscribeParams{URI: uri}); err != nil {
		_ = session.Close()
		return err
	}
	if !ss.subscriptionDesired(uri) {
		_ = session.Close()
		return nil
	}

	subscription := &resourceSubscription{client: client, session: session}
	ss.subscriptionsMu.Lock()
	if _, ok := ss.subscriptions[uri]; ok {
		ss.subscriptionsMu.Unlock()
		_ = session.Close()
		return nil
	}
	ss.subscriptions[uri] = subscription
	ss.subscriptionsMu.Unlock()
	go m.watchSubscription(ss, uri, session)
	return nil
}

func (m *Manager) unsubscribeModern(ctx context.Context, ss *serverSession, uri string) error {
	ss.subscriptionsMu.Lock()
	subscription, ok := ss.subscriptions[uri]
	if ok {
		delete(ss.subscriptions, uri)
	}
	ss.subscriptionsMu.Unlock()
	if !ok {
		return nil
	}

	// Closing the dedicated client cancels its subscriptions/listen stream and
	// leaves the main session's catalog listener untouched. Unsubscribe is
	// still issued first so servers that observe the explicit cancellation can
	// release their resource-side state promptly.
	var errs []error
	if err := subscription.session.Unsubscribe(ctx, &sdkmcp.UnsubscribeParams{URI: uri}); err != nil {
		errs = append(errs, err)
	}
	if err := subscription.session.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (m *Manager) restoreSubscriptions(ctx context.Context, ss *serverSession) error {
	if m == nil || ss == nil || ss.currentSession() == nil {
		return nil
	}
	uris := ss.desiredSubscriptionURIs()
	if len(uris) == 0 {
		return nil
	}
	if usesModernProtocol(ss) {
		stale := ss.takeSubscriptions()
		var errs []error
		for _, subscription := range stale {
			if subscription != nil && subscription.session != nil {
				if err := subscription.session.Close(); err != nil {
					errs = append(errs, err)
				}
			}
		}
		for _, uri := range uris {
			if !ss.subscriptionDesired(uri) {
				continue
			}
			subscribeCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
			err := m.subscribeModern(subscribeCtx, ss, uri)
			cancel()
			if err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	session := ss.currentSession()
	var errs []error
	for _, uri := range uris {
		if !ss.subscriptionDesired(uri) || session == nil {
			continue
		}
		subscribeCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
		err := session.Subscribe(subscribeCtx, &sdkmcp.SubscribeParams{URI: uri})
		cancel()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) watchSubscription(ss *serverSession, uri string, session *sdkmcp.ClientSession) {
	if ss == nil || session == nil {
		return
	}
	err := session.Wait()
	if m == nil || m.closed.Load() {
		return
	}
	if !ss.removeSubscriptionIf(uri, session) {
		return
	}
	params := map[string]any{"server": ss.cfg.Name, "uri": uri}
	if err != nil {
		params["err"] = err.Error()
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "MCP resource subscription lost", logging.LogOptions{Params: params})
	if ss.subscriptionDesired(uri) {
		go m.recoverSubscription(ss, uri)
	}
}

func (m *Manager) recoverSubscription(ss *serverSession, uri string) {
	delay := time.Second
	for attempt := 0; ; attempt++ {
		if m == nil || m.closed.Load() || !ss.subscriptionDesired(uri) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeoutFor(ss.cfg, m.callTimeout))
		err := func() error {
			if _, err := m.ensureSession(ctx, ss); err != nil {
				return err
			}
			return m.subscribeModern(ctx, ss, uri)
		}()
		cancel()
		if err == nil {
			return
		}
		if attempt == 0 || attempt%5 == 0 {
			logging.Log(logging.WARNING_LOG_LEVEL, "MCP resource subscription recovery failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "uri": uri, "err": err.Error()}})
		}
		if !waitForSessionRecovery(m, delay) {
			return
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
}

func waitForSessionRecovery(m *Manager, delay time.Duration) bool {
	if m == nil {
		return false
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return !m.closed.Load()
	case <-m.done:
		return false
	}
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
