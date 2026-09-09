package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/openai/openai-go/v2"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultCallTimeout    = 120 * time.Second
)

type Manager struct {
	mu          sync.RWMutex
	servers     []*serverSession
	registry    map[string]*remoteBinding
	tools       []RemoteTool
	resources   []RemoteResource
	templates   []RemoteResourceTemplate
	prompts     []RemotePrompt
	callTimeout time.Duration

	cfg        *Config
	stderr     io.Writer
	options    ManagerOptions
	connectMu  sync.Mutex
	connected  bool
	ready      chan struct{}
	connectErr error

	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
	done      chan struct{}

	oauthMu       sync.Mutex
	oauthHandlers map[string]auth.OAuthHandler
	oauthReady    map[string]bool
}

type remoteBinding struct {
	server *serverSession
	tool   RemoteTool
}

func StartLazy(stderr io.Writer) (*Manager, error) {
	return StartLazyWithOptions(stderr, nil)
}

func StartLazyWithOptions(stderr io.Writer, options *ManagerOptions) (*Manager, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return newManager(cfg, stderr, options), nil
}

func Start(ctx context.Context, stderr io.Writer) (*Manager, error) {
	return StartWithOptions(ctx, stderr, nil)
}

func StartWithOptions(ctx context.Context, stderr io.Writer, options *ManagerOptions) (*Manager, error) {
	m, err := StartLazyWithOptions(stderr, options)
	if err != nil {
		return nil, err
	}
	_, _, err = m.Connect(ctx)
	return m, err
}

func NewManager(ctx context.Context, cfg *Config, stderr io.Writer) *Manager {
	return NewManagerWithOptions(ctx, cfg, stderr, nil)
}

func NewManagerWithOptions(ctx context.Context, cfg *Config, stderr io.Writer, options *ManagerOptions) *Manager {
	m := newManager(cfg, stderr, options)
	if cfg == nil || len(cfg.Servers) == 0 {
		logging.Log(logging.INFO_LOG_LEVEL, "MCP config loaded without servers")
		close(m.ready)
		m.connected = true
		return m
	}
	for _, sc := range cfg.Servers {
		m.connectServer(ctx, sc, stderr)
	}
	close(m.ready)
	m.connected = true
	logging.Log(logging.INFO_LOG_LEVEL, "MCP manager initialized", logging.LogOptions{Params: map[string]any{"servers": len(m.servers), "tools": len(m.tools)}})
	return m
}

func newManager(cfg *Config, stderr io.Writer, options *ManagerOptions) *Manager {
	var opts ManagerOptions
	if options != nil {
		opts = *options
	}
	m := &Manager{
		registry:      map[string]*remoteBinding{},
		callTimeout:   defaultCallTimeout,
		cfg:           cfg,
		stderr:        stderr,
		options:       opts,
		ready:         make(chan struct{}),
		done:          make(chan struct{}),
		oauthHandlers: map[string]auth.OAuthHandler{},
		oauthReady:    map[string]bool{},
	}
	return m
}

func (m *Manager) Connect(ctx context.Context) (servers int, tools int, err error) {
	if m == nil {
		return 0, 0, nil
	}
	if m.closed.Load() {
		return 0, 0, ErrManagerClosed
	}
	m.connectMu.Lock()
	defer m.connectMu.Unlock()
	if m.connected {
		m.mu.RLock()
		servers, tools := len(m.servers), len(m.tools)
		m.mu.RUnlock()
		return servers, tools, m.connectErr
	}
	defer func() {
		close(m.ready)
		m.connected = true
		m.connectErr = err
	}()
	if m.cfg == nil || len(m.cfg.Servers) == 0 {
		logging.Log(logging.INFO_LOG_LEVEL, "MCP config loaded without servers")
		return 0, 0, nil
	}
	for _, sc := range m.cfg.Servers {
		m.connectServer(ctx, sc, m.stderr)
	}
	m.mu.RLock()
	servers, tools = len(m.servers), len(m.tools)
	m.mu.RUnlock()
	logging.Log(logging.INFO_LOG_LEVEL, "MCP manager initialized", logging.LogOptions{Params: map[string]any{"servers": servers, "tools": tools}})
	return servers, tools, nil
}

func (m *Manager) WaitReady(ctx context.Context) error {
	if m == nil || m.ready == nil {
		return nil
	}
	select {
	case <-m.ready:
		return m.connectErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) IsReady() bool {
	if m == nil || m.ready == nil {
		return true
	}
	select {
	case <-m.ready:
		return true
	default:
		return false
	}
}

func NewManagerWithRemoteTools(tools []RemoteTool) *Manager {
	m := &Manager{
		tools:         tools,
		registry:      map[string]*remoteBinding{},
		ready:         make(chan struct{}),
		done:          make(chan struct{}),
		oauthHandlers: map[string]auth.OAuthHandler{},
		oauthReady:    map[string]bool{},
	}
	close(m.ready)
	m.connected = true
	for i := range tools {
		t := tools[i]
		t.Schema = tooling.SchemaWithRequiredToolIntent(t.Schema)
		m.tools[i] = t
		m.registry[t.OpenAIName] = &remoteBinding{tool: t}
	}
	return m
}

func (m *Manager) connectServer(ctx context.Context, sc ServerConfig, stderr io.Writer) {
	if sc.Internal && strings.EqualFold(strings.TrimSpace(sc.Adapter), "cloak") {
		logging.Log(logging.INFO_LOG_LEVEL, "legacy Cloak MCP server ignored; Solomon uses the native adapter", logging.LogOptions{Params: map[string]any{"server": sc.Name}})
		return
	}
	ss := newServerSession(sc)
	session, err := m.openAndInstallSession(ctx, ss, stderr)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server connect failed", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type, "err": err.Error()}})
		return
	}
	m.hydrateServer(ctx, ss, session)
	m.mu.Lock()
	m.servers = append(m.servers, ss)
	m.mu.Unlock()
	logging.Log(logging.INFO_LOG_LEVEL, "MCP server connected", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type}})
}

func (m *Manager) openAndInstallSession(ctx context.Context, ss *serverSession, stderr io.Writer) (*sdkmcp.ClientSession, error) {
	if m == nil || ss == nil {
		return nil, ErrServerUnavailable
	}
	if m.closed.Load() {
		return nil, ErrManagerClosed
	}
	connectCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
	defer cancel()
	clientOptions := m.clientOptions(ss)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "solomon", Version: "dev", Title: "Solomon"}, &clientOptions)
	for _, root := range m.rootsSnapshot() {
		if root != nil {
			client.AddRoots(root)
		}
	}
	transport, err := m.transportFor(connectCtx, ss.cfg, stderr)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server transport setup failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "transport": ss.cfg.Type, "err": err.Error()}})
		return nil, err
	}
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, err
	}
	if m.closed.Load() {
		_ = session.Close()
		return nil, ErrManagerClosed
	}
	ss.installSession(client, session)
	go m.watchSession(ss, session)
	return session, nil
}

func (m *Manager) transportFor(ctx context.Context, sc ServerConfig, stderr io.Writer) (sdkmcp.Transport, error) {
	if sc.Type == TransportStreamableHTTP {
		oauthHandler, err := m.oauthHandlerFor(ctx, sc)
		if err != nil {
			return nil, err
		}
		transport := &sdkmcp.StreamableClientTransport{
			Endpoint:   sc.URL,
			HTTPClient: httpClientWithHeaders(sc.Headers),
			MaxRetries: 5,
		}
		if oauthHandler != nil {
			transport.OAuthHandler = oauthHandler
		}
		return transport, nil
	}
	if sc.Type == TransportSSE {
		return &persistentSSETransport{
			transport: sdkmcp.SSEClientTransport{
				Endpoint:   sc.URL,
				HTTPClient: httpClientWithHeaders(sc.Headers),
			},
		}, nil
	}
	return &commandTransport{
		command: sc.Command,
		args:    sc.Args,
		cwd:     sc.CWD,
		env:     sc.Env,
		stderr:  stderr,
	}, nil
}

// oauthHandlerFor returns one handler per configured server. The handler owns
// the token source, including refresh state, so reconnecting an MCP session
// does not restart the OAuth flow. Persistence across Solomon processes stays
// with the host-provided handler/factory and is never written to mcp.json.
func (m *Manager) oauthHandlerFor(ctx context.Context, sc ServerConfig) (auth.OAuthHandler, error) {
	if m == nil || (m.options.OAuthHandler == nil && sc.OAuth == nil) {
		return nil, nil
	}
	m.oauthMu.Lock()
	defer m.oauthMu.Unlock()
	if m.oauthReady[sc.Name] {
		return m.oauthHandlers[sc.Name], nil
	}
	var (
		h   auth.OAuthHandler
		err error
	)
	if m.options.OAuthHandler != nil {
		h, err = m.options.OAuthHandler(ctx, sc)
	} else {
		h, err = m.oauthHandler(ctx, sc)
	}
	if err != nil {
		return nil, err
	}
	m.oauthHandlers[sc.Name] = h
	m.oauthReady[sc.Name] = true
	return h, nil
}

func (m *Manager) oauthHandler(_ context.Context, sc ServerConfig) (auth.OAuthHandler, error) {
	if sc.OAuth == nil {
		return nil, nil
	}
	if m.options.AuthorizationCodeFetcher == nil {
		return nil, fmt.Errorf("MCP server %q declares OAuth but no AuthorizationCodeFetcher was configured", sc.Name)
	}
	oauthCfg := sc.OAuth
	handlerCfg := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              oauthCfg.RedirectURL,
		AuthorizationCodeFetcher: m.options.AuthorizationCodeFetcher,
		RequestRefreshToken:      oauthCfg.RequestRefresh,
		Client:                   m.options.OAuthHTTPClient,
	}
	if oauthCfg.ClientIDMetadataURL != "" {
		handlerCfg.ClientIDMetadataDocumentConfig = &auth.ClientIDMetadataDocumentConfig{URL: oauthCfg.ClientIDMetadataURL}
	}
	if oauthCfg.ClientID != "" {
		handlerCfg.PreregisteredClient = &oauthex.ClientCredentials{ClientID: oauthCfg.ClientID, ClientSecretAuth: nil}
		if oauthCfg.ClientSecret != "" {
			handlerCfg.PreregisteredClient.ClientSecretAuth = &oauthex.ClientSecretAuth{ClientSecret: oauthCfg.ClientSecret}
		}
	}
	if len(oauthCfg.RedirectURIs) > 0 {
		handlerCfg.DynamicClientRegistrationConfig = &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				RedirectURIs: oauthCfg.RedirectURIs,
				ClientName:   oauthCfg.ClientName,
				ClientURI:    oauthCfg.ClientURI,
				Scope:        oauthCfg.Scope,
			},
		}
	}
	return auth.NewAuthorizationCodeHandler(handlerCfg)
}

func (m *Manager) OpenAITools() []openai.ChatCompletionToolUnionParam {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.tools) == 0 {
		return nil
	}
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(m.tools))
	for _, tool := range m.tools {
		if m.serverInternalLocked(tool.ServerName) {
			continue
		}
		out = append(out, OpenAITool(tool))
	}
	return out
}

func (m *Manager) ToolDump() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.tools) == 0 {
		return ""
	}
	var b strings.Builder
	publicIndex := 0
	for _, tool := range m.tools {
		if m.serverInternalLocked(tool.ServerName) {
			continue
		}
		if publicIndex > 0 {
			b.WriteString("\n---\n")
		}
		publicIndex++
		schema, err := json.Marshal(mcpArgumentsSchema(tool.Schema))
		if err != nil {
			schema = []byte(`{"type":"object","properties":{}}`)
		}
		b.WriteString(fmt.Sprintf("name: %s\ndescription: %s\nsdk_call: sdk.mcp.<tool>(intent, args)\nparameters: %s\n", tool.OpenAIName, tool.Description, string(schema)))
	}
	return b.String()
}

func (m *Manager) HasTool(name string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	binding, ok := m.registry[name]
	if ok && binding != nil && binding.server != nil && binding.server.cfg.Internal {
		ok = false
	}
	m.mu.RUnlock()
	return ok
}

func (m *Manager) CallTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if m == nil {
		return nil, fmt.Errorf("MCP manager unavailable")
	}
	if err := tooling.ValidateToolIntent(raw); err != nil {
		return nil, fmt.Errorf("MCP tool %q: %w", name, err)
	}
	if err := m.WaitReady(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	binding, ok := m.registry[name]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown MCP tool %q", name)
	}
	if binding.server != nil && binding.server.cfg.Internal {
		return nil, fmt.Errorf("MCP tool %q is internal and cannot be called through the generic MCP surface", name)
	}
	return m.callToolBinding(ctx, name, binding, raw)
}

// CallInternalTool invokes a host-managed MCP tool by its stable server and
// remote tool names. Internal adapter calls deliberately bypass the
// model-facing MCP catalog, but still use the Manager's session lifecycle,
// retry policy, and logging.
func (m *Manager) CallInternalTool(ctx context.Context, serverName, toolName, intent string, args map[string]any) (any, error) {
	if m == nil {
		return nil, fmt.Errorf("MCP manager unavailable")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil, fmt.Errorf("MCP internal tool: server name is required")
	}
	if strings.TrimSpace(toolName) == "" {
		return nil, fmt.Errorf("MCP internal tool: tool name is required")
	}
	if strings.TrimSpace(intent) == "" {
		return nil, fmt.Errorf("MCP internal tool %q: intent is required", toolName)
	}
	if err := m.WaitReady(ctx); err != nil {
		return nil, err
	}
	binding, err := m.internalToolBinding(ctx, serverName, toolName)
	if err != nil {
		return nil, err
	}

	callArgs := make(map[string]any, len(args)+1)
	for key, value := range args {
		if key != "intent" {
			callArgs[key] = value
		}
	}
	callArgs["intent"] = intent
	raw, err := json.Marshal(callArgs)
	if err != nil {
		return nil, fmt.Errorf("MCP internal tool %q arguments: %w", toolName, err)
	}
	return m.callToolBinding(ctx, binding.tool.OpenAIName, binding, raw)
}

func (m *Manager) internalToolBinding(ctx context.Context, serverName, toolName string) (*remoteBinding, error) {
	m.mu.RLock()
	var server *serverSession
	for _, candidate := range m.servers {
		if candidate != nil && candidate.cfg.Name == serverName {
			server = candidate
			break
		}
	}
	m.mu.RUnlock()
	if server == nil {
		return nil, fmt.Errorf("unknown internal MCP server %q", serverName)
	}
	if !server.cfg.Internal {
		return nil, fmt.Errorf("MCP server %q is not internal", serverName)
	}
	if _, err := m.ensureSession(ctx, server); err != nil {
		return nil, unavailableError(serverName, "internal tools/call", err)
	}

	lookup := func() *remoteBinding {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for _, tool := range server.tools {
			if tool.ToolName == toolName {
				toolCopy := tool
				return &remoteBinding{server: server, tool: toolCopy}
			}
		}
		return nil
	}
	if binding := lookup(); binding != nil {
		return binding, nil
	}
	if err := m.refreshTools(ctx, server); err != nil {
		return nil, unavailableError(serverName, "internal tools/list", err)
	}
	if binding := lookup(); binding != nil {
		return binding, nil
	}
	return nil, fmt.Errorf("unknown internal MCP tool %q on server %q", toolName, serverName)
}

func (m *Manager) callToolBinding(ctx context.Context, name string, binding *remoteBinding, raw json.RawMessage) (any, error) {
	if binding == nil {
		return nil, fmt.Errorf("MCP tool %q unavailable", name)
	}
	if binding.server == nil {
		return nil, unavailableError("", "tool", ErrServerUnavailable)
	}
	args := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("MCP tool arguments must be an object: %w", err)
		}
	}
	intent, _ := tooling.ToolIntent(raw)
	delete(args, "intent")
	start := time.Now()
	logFailure := func(err error) (any, error) {
		params := map[string]any{"server": binding.server.cfg.Name, "tool": binding.tool.ToolName, "openai_tool": name, "elapsed_ms": time.Since(start).Milliseconds(), "err": err.Error()}
		if intent != "" {
			params["intent"] = intent
		}
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP tools/call failed", logging.LogOptions{Params: params})
		return nil, err
	}
	call := func(session *sdkmcp.ClientSession) (*sdkmcp.CallToolResult, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeoutFor(binding.server.cfg, m.callTimeout))
		defer cancel()
		return session.CallTool(callCtx, &sdkmcp.CallToolParams{Name: binding.tool.ToolName, Arguments: args})
	}
	session, err := m.ensureSession(ctx, binding.server)
	if err != nil {
		return logFailure(unavailableError(binding.server.cfg.Name, "tools/call", err))
	}
	res, err := call(session)
	if err != nil && isSessionFailure(err) {
		m.invalidateAndCloseSession(binding.server, session, err)
		if m.toolCallMayRetry(binding.server.cfg.Name, binding.tool.ToolName, binding.tool.Definition, err) {
			session, reconnectErr := m.ensureSession(ctx, binding.server)
			if reconnectErr != nil {
				return logFailure(unknownOutcomeError(name, errors.Join(err, reconnectErr)))
			}
			res, err = call(session)
			if err != nil && isSessionFailure(err) {
				m.invalidateAndCloseSession(binding.server, session, err)
				return logFailure(unknownOutcomeError(name, err))
			}
		} else {
			// Reconnect for the next operation, but never replay a call whose
			// delivery status is uncertain (including multi-round-trip calls).
			if _, reconnectErr := m.ensureSession(ctx, binding.server); reconnectErr != nil {
				err = errors.Join(err, reconnectErr)
			}
			return logFailure(unknownOutcomeError(name, err))
		}
	}
	elapsed := time.Since(start)
	params := map[string]any{"server": binding.server.cfg.Name, "tool": binding.tool.ToolName, "openai_tool": name, "elapsed_ms": elapsed.Milliseconds()}
	if intent != "" {
		params["intent"] = intent
	}
	if err != nil {
		params["err"] = err.Error()
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP tools/call failed", logging.LogOptions{Params: params})
		return nil, err
	}
	logging.Log(logging.INFO_LOG_LEVEL, "MCP tools/call completed", logging.LogOptions{Params: params})
	return convertResult(res)
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		if m.done != nil {
			close(m.done)
		}
		m.mu.RLock()
		servers := append([]*serverSession(nil), m.servers...)
		m.mu.RUnlock()
		var errs []error
		for _, server := range servers {
			if err := server.closeSubscriptions(); err != nil {
				errs = append(errs, err)
			}
			if session := server.currentSession(); session != nil {
				if err := session.Close(); err != nil {
					errs = append(errs, err)
				}
				logging.Log(logging.INFO_LOG_LEVEL, "MCP server disconnected", logging.LogOptions{Params: map[string]any{"server": server.cfg.Name}})
			}
		}
		m.closeErr = errors.Join(errs...)
	})
	return m.closeErr
}

func convertResult(res *sdkmcp.CallToolResult) (any, error) {
	if res == nil {
		return nil, nil
	}
	content := contentValues(res.Content)
	if res.NeedsInput() {
		return map[string]any{
			"resultType":    "input_required",
			"inputRequests": res.InputRequests,
			"requestState":  res.RequestState,
			"content":       content,
		}, nil
	}
	if res.IsError {
		return map[string]any{"error": contentText(content)}, nil
	}
	if res.StructuredContent != nil {
		return map[string]any{"structuredContent": res.StructuredContent, "content": content}, nil
	}
	if len(content) == 1 {
		return content[0], nil
	}
	return content, nil
}

func contentValues(items []sdkmcp.Content) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case *sdkmcp.TextContent:
			out = append(out, v.Text)
		default:
			b, err := item.MarshalJSON()
			if err != nil {
				out = append(out, map[string]any{"error": err.Error()})
				continue
			}
			var decoded any
			if err := json.Unmarshal(b, &decoded); err != nil {
				out = append(out, string(b))
				continue
			}
			out = append(out, decoded)
		}
	}
	return out
}

func contentText(items []any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			parts = append(parts, s)
			continue
		}
		b, _ := json.Marshal(item)
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n")
}

func timeoutFor(sc ServerConfig, fallback time.Duration) time.Duration {
	if sc.Timeout <= 0 {
		return fallback
	}
	return time.Duration(sc.Timeout) * time.Millisecond
}
