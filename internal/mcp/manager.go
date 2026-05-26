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

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v2"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultCallTimeout    = 120 * time.Second
)

type Manager struct {
	servers     []*serverSession
	registry    map[string]*remoteBinding
	tools       []RemoteTool
	callTimeout time.Duration

	cfg       *Config
	stderr    io.Writer
	connectMu sync.Mutex
	connected bool
	ready     chan struct{}
	connectErr error
}

type serverSession struct {
	cfg     ServerConfig
	client  *sdkmcp.Client
	session *sdkmcp.ClientSession
}

type remoteBinding struct {
	server *serverSession
	tool   RemoteTool
}

func StartLazy(stderr io.Writer) (*Manager, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return &Manager{
		registry:    map[string]*remoteBinding{},
		callTimeout: defaultCallTimeout,
		cfg:         cfg,
		stderr:      stderr,
		ready:       make(chan struct{}),
	}, nil
}

func Start(ctx context.Context, stderr io.Writer) (*Manager, error) {
	m, err := StartLazy(stderr)
	if err != nil {
		return nil, err
	}
	_, _, err = m.Connect(ctx)
	return m, err
}

func NewManager(ctx context.Context, cfg *Config, stderr io.Writer) *Manager {
	m := &Manager{
		registry:    map[string]*remoteBinding{},
		callTimeout: defaultCallTimeout,
		cfg:         cfg,
		stderr:      stderr,
		ready:       make(chan struct{}),
	}
	if cfg == nil || len(cfg.Servers) == 0 {
		logging.Log(logging.INFO_LOG_LEVEL, "MCP config loaded without servers")
		close(m.ready)
		m.connected = true
		return m
	}
	usedNames := map[string]bool{}
	for _, sc := range cfg.Servers {
		m.connectServer(ctx, sc, stderr, usedNames)
	}
	close(m.ready)
	m.connected = true
	logging.Log(logging.INFO_LOG_LEVEL, "MCP manager initialized", logging.LogOptions{Params: map[string]any{"servers": len(m.servers), "tools": len(m.tools)}})
	return m
}

func (m *Manager) Connect(ctx context.Context) (servers int, tools int, err error) {
	if m == nil {
		return 0, 0, nil
	}
	m.connectMu.Lock()
	defer m.connectMu.Unlock()
	if m.connected {
		return len(m.servers), len(m.tools), m.connectErr
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
	usedNames := map[string]bool{}
	for _, sc := range m.cfg.Servers {
		m.connectServer(ctx, sc, m.stderr, usedNames)
	}
	logging.Log(logging.INFO_LOG_LEVEL, "MCP manager initialized", logging.LogOptions{Params: map[string]any{"servers": len(m.servers), "tools": len(m.tools)}})
	return len(m.servers), len(m.tools), nil
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
	return &Manager{tools: tools}
}

func (m *Manager) connectServer(ctx context.Context, sc ServerConfig, stderr io.Writer, usedNames map[string]bool) {
	connectCtx, cancel := context.WithTimeout(ctx, timeoutFor(sc, defaultConnectTimeout))
	defer cancel()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "solomon", Version: "dev"}, nil)
	transport := transportFor(sc, stderr)
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server connect failed", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type, "err": err.Error()}})
		return
	}
	ss := &serverSession{cfg: sc, client: client, session: session}
	if err := m.registerTools(ctx, ss, usedNames); err != nil {
		_ = session.Close()
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server tools/list failed", logging.LogOptions{Params: map[string]any{"server": sc.Name, "err": err.Error()}})
		return
	}
	m.servers = append(m.servers, ss)
	logging.Log(logging.INFO_LOG_LEVEL, "MCP server connected", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type}})
}

func transportFor(sc ServerConfig, stderr io.Writer) sdkmcp.Transport {
	if sc.Type == TransportStreamableHTTP {
		return &sdkmcp.StreamableClientTransport{
			Endpoint:   sc.URL,
			HTTPClient: httpClientWithHeaders(sc.Headers),
			MaxRetries: -1,
		}
	}
	return &commandTransport{
		command: sc.Command,
		args:    sc.Args,
		cwd:     sc.CWD,
		env:     sc.Env,
		stderr:  stderr,
	}
}

func (m *Manager) registerTools(ctx context.Context, ss *serverSession, usedNames map[string]bool) error {
	cursor := ""
	for {
		listCtx, cancel := context.WithTimeout(ctx, timeoutFor(ss.cfg, defaultConnectTimeout))
		res, err := ss.session.ListTools(listCtx, &sdkmcp.ListToolsParams{Cursor: cursor})
		cancel()
		if err != nil {
			return err
		}
		for _, tool := range res.Tools {
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
			base := ExposedToolName(ss.cfg.Name, tool.Name)
			rt.OpenAIName = UniqueToolName(base, usedNames)
			m.registry[rt.OpenAIName] = &remoteBinding{server: ss, tool: rt}
			m.tools = append(m.tools, rt)
			logging.Log(logging.INFO_LOG_LEVEL, "MCP tool registered", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "tool": tool.Name, "openai_tool": rt.OpenAIName}})
		}
		if strings.TrimSpace(res.NextCursor) == "" {
			return nil
		}
		cursor = res.NextCursor
	}
}

func (m *Manager) OpenAITools() []openai.ChatCompletionToolUnionParam {
	if m == nil || len(m.tools) == 0 {
		return nil
	}
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(m.tools))
	for _, tool := range m.tools {
		out = append(out, OpenAITool(tool))
	}
	return out
}

func (m *Manager) ToolDump() string {
	if m == nil || len(m.tools) == 0 {
		return ""
	}
	var b strings.Builder
	for i, tool := range m.tools {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		schema, err := json.Marshal(tool.Schema)
		if err != nil {
			schema = []byte(`{"type":"object","properties":{}}`)
		}
		b.WriteString(fmt.Sprintf("name: %s\ndescription: %s\nsignature: %s\n", tool.OpenAIName, tool.Description, string(schema)))
	}
	return b.String()
}

func (m *Manager) HasTool(name string) bool {
	if m == nil {
		return false
	}
	_, ok := m.registry[name]
	return ok
}

func (m *Manager) CallTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if m == nil {
		return nil, fmt.Errorf("MCP manager unavailable")
	}
	if err := m.WaitReady(ctx); err != nil {
		return nil, err
	}
	binding, ok := m.registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown MCP tool %q", name)
	}
	args := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("MCP tool arguments must be an object: %w", err)
		}
	}
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
	var errs []error
	for _, server := range m.servers {
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
