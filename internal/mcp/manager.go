package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
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
}

type serverSession struct {
	cfg             ServerConfig
	client          *sdkmcp.Client
	session         *sdkmcp.ClientSession
	tools           []RemoteTool
	resources       []RemoteResource
	templates       []RemoteResourceTemplate
	prompts         []RemotePrompt
	subscriptionsMu sync.Mutex
	subscriptions   map[string]*resourceSubscription
}

type resourceSubscription struct {
	client  *sdkmcp.Client
	session *sdkmcp.ClientSession
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
		registry:    map[string]*remoteBinding{},
		callTimeout: defaultCallTimeout,
		cfg:         cfg,
		stderr:      stderr,
		options:     opts,
		ready:       make(chan struct{}),
	}
	return m
}

func (m *Manager) Connect(ctx context.Context) (servers int, tools int, err error) {
	if m == nil {
		return 0, 0, nil
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
		tools:    tools,
		registry: map[string]*remoteBinding{},
		ready:    make(chan struct{}),
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
	connectCtx, cancel := context.WithTimeout(ctx, timeoutFor(sc, defaultConnectTimeout))
	defer cancel()
	var ss *serverSession
	clientOptions := m.clientOptions(&ss)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "solomon", Version: "dev", Title: "Solomon"}, &clientOptions)
	for _, root := range m.options.Roots {
		if root != nil {
			client.AddRoots(root)
		}
	}
	transport, err := m.transportFor(connectCtx, sc, stderr)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server transport setup failed", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type, "err": err.Error()}})
		return
	}
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server connect failed", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type, "err": err.Error()}})
		return
	}
	ss = &serverSession{cfg: sc, client: client, session: session, subscriptions: map[string]*resourceSubscription{}}
	if err := m.registerServerCatalog(ctx, ss); err != nil {
		_ = session.Close()
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server catalog failed", logging.LogOptions{Params: map[string]any{"server": sc.Name, "err": err.Error()}})
		return
	}
	m.mu.Lock()
	m.servers = append(m.servers, ss)
	m.mu.Unlock()
	logging.Log(logging.INFO_LOG_LEVEL, "MCP server connected", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type}})
}

func (m *Manager) transportFor(ctx context.Context, sc ServerConfig, stderr io.Writer) (sdkmcp.Transport, error) {
	if sc.Type == TransportStreamableHTTP {
		var oauthHandler auth.OAuthHandler
		if m.options.OAuthHandler != nil {
			h, err := m.options.OAuthHandler(ctx, sc)
			if err != nil {
				return nil, err
			}
			oauthHandler = h
		} else if sc.OAuth != nil {
			h, err := m.oauthHandler(ctx, sc)
			if err != nil {
				return nil, err
			}
			oauthHandler = h
		}
		transport := &sdkmcp.StreamableClientTransport{
			Endpoint:   sc.URL,
			HTTPClient: httpClientWithHeaders(sc.Headers),
			MaxRetries: -1,
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
	for i, tool := range m.tools {
		if i > 0 {
			b.WriteString("\n---\n")
		}
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
	defer m.mu.RUnlock()
	_, ok := m.registry[name]
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
	if binding.server == nil || binding.server.session == nil {
		return nil, fmt.Errorf("MCP tool %q is not connected", name)
	}
	args := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("MCP tool arguments must be an object: %w", err)
		}
	}
	delete(args, "intent")
	callCtx, cancel := context.WithTimeout(ctx, timeoutFor(binding.server.cfg, m.callTimeout))
	defer cancel()
	start := time.Now()
	res, err := binding.server.session.CallTool(callCtx, &sdkmcp.CallToolParams{Name: binding.tool.ToolName, Arguments: args})
	elapsed := time.Since(start)
	params := map[string]any{"server": binding.server.cfg.Name, "tool": binding.tool.ToolName, "openai_tool": name, "elapsed_ms": elapsed.Milliseconds()}
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
	m.mu.RLock()
	servers := append([]*serverSession(nil), m.servers...)
	m.mu.RUnlock()
	var errs []error
	for _, server := range servers {
		if err := server.closeSubscriptions(); err != nil {
			errs = append(errs, err)
		}
		if server.session != nil {
			if err := server.session.Close(); err != nil {
				errs = append(errs, err)
			}
			logging.Log(logging.INFO_LOG_LEVEL, "MCP server disconnected", logging.LogOptions{Params: map[string]any{"server": server.cfg.Name}})
		}
	}
	return errors.Join(errs...)
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
