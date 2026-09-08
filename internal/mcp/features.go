package mcp

import (
	"context"
	"errors"
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

func (m *Manager) clientOptions(ss **serverSession) sdkmcp.ClientOptions {
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
		if ss != nil && *ss != nil {
			m.refreshCatalogAsync(ctx, *ss, "tools", m.refreshTools)
		}
	}
	opts.PromptListChangedHandler = func(ctx context.Context, req *sdkmcp.PromptListChangedRequest) {
		if userPromptChanged != nil {
			userPromptChanged(ctx, req)
		}
		if ss != nil && *ss != nil {
			m.refreshCatalogAsync(ctx, *ss, "prompts", m.refreshPrompts)
		}
	}
	opts.ResourceListChangedHandler = func(ctx context.Context, req *sdkmcp.ResourceListChangedRequest) {
		if userResourceChanged != nil {
			userResourceChanged(ctx, req)
		}
		if ss != nil && *ss != nil {
			refreshCtx := context.WithoutCancel(ctx)
			go func(server *serverSession) {
				if err := m.refreshResources(refreshCtx, server); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "MCP resources catalog refresh failed", logging.LogOptions{Params: map[string]any{"server": server.cfg.Name, "err": err.Error()}})
				}
				if err := m.refreshResourceTemplates(refreshCtx, server); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "MCP resource templates refresh failed", logging.LogOptions{Params: map[string]any{"server": server.cfg.Name, "err": err.Error()}})
				}
			}(*ss)
		}
	}
	opts.ResourceUpdatedHandler = func(ctx context.Context, req *sdkmcp.ResourceUpdatedNotificationRequest) {
		if userResourceUpdated != nil {
			userResourceUpdated(ctx, req)
		}
	}
	return opts
}

// subscriptionClientOptions creates options for a dedicated modern resource
// subscription session. The main session owns the July subscriptions/listen
// stream for catalog changes; resource subscriptions must not share that
// session because the SDK cancels each resource listener independently.
func (m *Manager) subscriptionClientOptions(ss **serverSession) sdkmcp.ClientOptions {
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
	if ss == nil || ss.session == nil {
		return false
	}
	result := ss.session.InitializeResult()
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
	if !supportsFeature(ss, "tools") {
		m.replaceServerTools(ss, nil)
		return nil
	}
	var all []*sdkmcp.Tool
	cursor := ""
	for {
		listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
		res, err := ss.session.ListTools(listCtx, &sdkmcp.ListToolsParams{Cursor: cursor})
		cancel()
		if err != nil {
			return err
		}
		all = append(all, res.Tools...)
		if strings.TrimSpace(res.NextCursor) == "" {
			break
		}
		cursor = res.NextCursor
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
	return nil
}

func (m *Manager) refreshResources(ctx context.Context, ss *serverSession) error {
	if !supportsFeature(ss, "resources") {
		m.replaceServerResources(ss, nil)
		return nil
	}
	var resources []*sdkmcp.Resource
	cursor := ""
	for {
		listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
		res, err := ss.session.ListResources(listCtx, &sdkmcp.ListResourcesParams{Cursor: cursor})
		cancel()
		if err != nil {
			return err
		}
		resources = append(resources, res.Resources...)
		if strings.TrimSpace(res.NextCursor) == "" {
			break
		}
		cursor = res.NextCursor
	}
	out := make([]RemoteResource, 0, len(resources))
	for _, resource := range resources {
		if resource != nil {
			out = append(out, RemoteResource{ServerName: ss.cfg.Name, Definition: resource})
		}
	}
	m.replaceServerResources(ss, out)
	return nil
}

func (m *Manager) refreshResourceTemplates(ctx context.Context, ss *serverSession) error {
	if !supportsFeature(ss, "resources") {
		m.replaceServerResourceTemplates(ss, nil)
		return nil
	}
	var templates []*sdkmcp.ResourceTemplate
	cursor := ""
	for {
		listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
		res, err := ss.session.ListResourceTemplates(listCtx, &sdkmcp.ListResourceTemplatesParams{Cursor: cursor})
		cancel()
		if err != nil {
			return err
		}
		templates = append(templates, res.ResourceTemplates...)
		if strings.TrimSpace(res.NextCursor) == "" {
			break
		}
		cursor = res.NextCursor
	}
	out := make([]RemoteResourceTemplate, 0, len(templates))
	for _, template := range templates {
		if template != nil {
			out = append(out, RemoteResourceTemplate{ServerName: ss.cfg.Name, Definition: template})
		}
	}
	m.replaceServerResourceTemplates(ss, out)
	return nil
}

func (m *Manager) refreshPrompts(ctx context.Context, ss *serverSession) error {
	if !supportsFeature(ss, "prompts") {
		m.replaceServerPrompts(ss, nil)
		return nil
	}
	var prompts []*sdkmcp.Prompt
	cursor := ""
	for {
		listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
		res, err := ss.session.ListPrompts(listCtx, &sdkmcp.ListPromptsParams{Cursor: cursor})
		cancel()
		if err != nil {
			return err
		}
		prompts = append(prompts, res.Prompts...)
		if strings.TrimSpace(res.NextCursor) == "" {
			break
		}
		cursor = res.NextCursor
	}
	out := make([]RemotePrompt, 0, len(prompts))
	for _, prompt := range prompts {
		if prompt != nil {
			out = append(out, RemotePrompt{ServerName: ss.cfg.Name, Definition: prompt})
		}
	}
	m.replaceServerPrompts(ss, out)
	return nil
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
	defer m.mu.RUnlock()
	for _, ss := range m.servers {
		if ss.cfg.Name == serverName {
			return ss, nil
		}
	}
	return nil, fmt.Errorf("unknown MCP server %q", serverName)
}

func (m *Manager) AddRoots(roots ...*sdkmcp.Root) {
	if m == nil || len(roots) == 0 {
		return
	}
	m.mu.RLock()
	servers := append([]*serverSession(nil), m.servers...)
	m.mu.RUnlock()
	for _, ss := range servers {
		ss.client.AddRoots(roots...)
		for _, client := range ss.subscriptionClients() {
			client.AddRoots(roots...)
		}
	}
}

func (m *Manager) RemoveRoots(uris ...string) {
	if m == nil || len(uris) == 0 {
		return
	}
	m.mu.RLock()
	servers := append([]*serverSession(nil), m.servers...)
	m.mu.RUnlock()
	for _, ss := range servers {
		ss.client.RemoveRoots(uris...)
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
		result := ss.session.InitializeResult()
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
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	return ss.session.ReadResource(callCtx, &sdkmcp.ReadResourceParams{URI: uri})
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
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	return ss.session.GetPrompt(callCtx, &sdkmcp.GetPromptParams{Name: name, Arguments: arguments})
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
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	return ss.session.Complete(callCtx, params)
}

// Subscribe subscribes to updates for a resource. For MCP July servers the
// SDK maps this to subscriptions/listen; for legacy servers it uses the
// resources/subscribe request.
func (m *Manager) Subscribe(ctx context.Context, serverName, uri string) error {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	if usesModernProtocol(ss) {
		return m.subscribeModern(callCtx, ss, uri)
	}
	return ss.session.Subscribe(callCtx, &sdkmcp.SubscribeParams{URI: uri})
}

// Unsubscribe stops a resource subscription.
func (m *Manager) Unsubscribe(ctx context.Context, serverName, uri string) error {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	if usesModernProtocol(ss) {
		return m.unsubscribeModern(callCtx, ss, uri)
	}
	return ss.session.Unsubscribe(callCtx, &sdkmcp.UnsubscribeParams{URI: uri})
}

func (m *Manager) subscribeModern(ctx context.Context, ss *serverSession, uri string) error {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return fmt.Errorf("MCP resource subscription requires a URI")
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

	var subscriptionServer *serverSession = ss
	clientOptions := m.subscriptionClientOptions(&subscriptionServer)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "solomon", Version: "dev", Title: "Solomon"}, &clientOptions)
	for _, root := range m.options.Roots {
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

	subscription := &resourceSubscription{client: client, session: session}
	ss.subscriptionsMu.Lock()
	if _, ok := ss.subscriptions[uri]; ok {
		ss.subscriptionsMu.Unlock()
		_ = session.Close()
		return nil
	}
	ss.subscriptions[uri] = subscription
	ss.subscriptionsMu.Unlock()
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

func (ss *serverSession) closeSubscriptions() error {
	if ss == nil {
		return nil
	}
	ss.subscriptionsMu.Lock()
	subscriptions := make([]*resourceSubscription, 0, len(ss.subscriptions))
	for uri, subscription := range ss.subscriptions {
		delete(ss.subscriptions, uri)
		subscriptions = append(subscriptions, subscription)
	}
	ss.subscriptionsMu.Unlock()
	var errs []error
	for _, subscription := range subscriptions {
		if subscription != nil && subscription.session != nil {
			if err := subscription.session.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (ss *serverSession) subscriptionClients() []*sdkmcp.Client {
	if ss == nil {
		return nil
	}
	ss.subscriptionsMu.Lock()
	defer ss.subscriptionsMu.Unlock()
	clients := make([]*sdkmcp.Client, 0, len(ss.subscriptions))
	for _, subscription := range ss.subscriptions {
		if subscription != nil && subscription.client != nil {
			clients = append(clients, subscription.client)
		}
	}
	return clients
}

// Ping checks the connection to one MCP server.
func (m *Manager) Ping(ctx context.Context, serverName string) error {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	if usesModernProtocol(ss) {
		return fmt.Errorf("MCP server %q does not support ping in protocol %s", serverName, ss.session.InitializeResult().ProtocolVersion)
	}
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	return ss.session.Ping(callCtx, nil)
}

// SetLoggingLevel requests a minimum log level from one MCP server. Logging
// is deprecated by MCP July but remains part of the compatibility surface.
func (m *Manager) SetLoggingLevel(ctx context.Context, serverName string, level sdkmcp.LoggingLevel) error {
	ss, err := m.sessionFor(ctx, serverName)
	if err != nil {
		return err
	}
	if usesModernProtocol(ss) {
		return fmt.Errorf("MCP server %q does not support logging level in protocol %s", serverName, ss.session.InitializeResult().ProtocolVersion)
	}
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, m.callTimeout))
	defer cancel()
	return ss.session.SetLoggingLevel(callCtx, &sdkmcp.SetLoggingLevelParams{Level: level})
}

func usesModernProtocol(ss *serverSession) bool {
	if ss == nil || ss.session == nil {
		return false
	}
	result := ss.session.InitializeResult()
	return result != nil && result.ProtocolVersion >= "2026-07-28"
}
