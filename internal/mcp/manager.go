package mcp

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
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

	closed          atomic.Bool
	closeOnce       sync.Once
	closeErr        error
	done            chan struct{}
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc

	oauthMu       sync.Mutex
	oauthHandlers map[string]auth.OAuthHandler
	oauthReady    map[string]bool
	rateMu        sync.Mutex
	rateLimits    map[string]time.Time
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
	_, _, _ = m.Connect(ctx)
	return m
}

func newManager(cfg *Config, stderr io.Writer, options *ManagerOptions) *Manager {
	var opts ManagerOptions
	if options != nil {
		opts = *options
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	m := &Manager{
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
		registry:        map[string]*remoteBinding{},
		callTimeout:     defaultCallTimeout,
		cfg:             cfg,
		stderr:          stderr,
		options:         opts,
		ready:           make(chan struct{}),
		done:            make(chan struct{}),
		oauthHandlers:   map[string]auth.OAuthHandler{},
		oauthReady:      map[string]bool{},
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
	if m.closed.Load() {
		return 0, 0, ErrManagerClosed
	}
	if m.connected {
		m.mu.RLock()
		servers, tools := len(m.servers), len(m.tools)
		m.mu.RUnlock()
		return servers, tools, m.connectErr
	}
	defer func() {
		m.connectErr = err
		m.connected = true
		close(m.ready)
	}()
	if m.cfg == nil || len(m.cfg.Servers) == 0 {
		logging.Log(logging.INFO_LOG_LEVEL, "MCP config loaded without servers")
		return 0, 0, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if m.lifecycleCtx != nil {
		stop := context.AfterFunc(m.lifecycleCtx, cancel)
		defer stop()
	}
	for _, sc := range m.cfg.Servers {
		if m.closed.Load() {
			return 0, 0, ErrManagerClosed
		}
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		m.connectServer(ctx, sc, m.stderr)
	}
	if m.closed.Load() {
		return 0, 0, ErrManagerClosed
	}
	if err := ctx.Err(); err != nil {
		return 0, 0, err
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
	if m.closed.Load() {
		return ErrManagerClosed
	}
	select {
	case <-m.done:
		return ErrManagerClosed
	case <-m.ready:
		if m.closed.Load() {
			return ErrManagerClosed
		}
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
	if m.lazyAdapter(sc) {
		m.mu.Lock()
		if !m.closed.Load() {
			m.servers = append(m.servers, ss)
		}
		m.mu.Unlock()
		return
	}
	session, err := m.openAndInstallSession(ctx, ss, stderr)
	if err != nil {
		if m.closed.Load() || ctx.Err() != nil {
			return
		}
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server connect failed", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type, "err": err.Error()}})
		return
	}
	m.hydrateServer(ctx, ss, session)
	if m.closed.Load() || ctx.Err() != nil {
		return
	}
	logging.Log(logging.INFO_LOG_LEVEL, "MCP server connected", logging.LogOptions{Params: map[string]any{"server": sc.Name, "transport": sc.Type}})
}

func (m *Manager) lazyAdapter(sc ServerConfig) bool {
	return m.options.LazyInternalAdapters && sc.Internal && (sc.Adapter == "exa" || sc.Adapter == "parallel")
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
	if m.lifecycleCtx != nil {
		stop := context.AfterFunc(m.lifecycleCtx, cancel)
		defer stop()
	}
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
	// Installation and shutdown share the lock: Close must see every session,
	// including one whose catalog is still being hydrated.
	m.mu.Lock()
	if m.closed.Load() {
		m.mu.Unlock()
		_ = session.Close()
		return nil, ErrManagerClosed
	}
	ss.installSession(client, session)
	registered := false
	for _, existing := range m.servers {
		if existing == ss {
			registered = true
			break
		}
	}
	if !registered {
		m.servers = append(m.servers, ss)
	}
	m.mu.Unlock()
	go m.watchSession(ss, session)
	return session, nil
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed.Store(true)
		if m.lifecycleCancel != nil {
			m.lifecycleCancel()
		}
		if m.done != nil {
			close(m.done)
		}
		servers := append([]*serverSession(nil), m.servers...)
		m.mu.Unlock()
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

func timeoutFor(sc ServerConfig, fallback time.Duration) time.Duration {
	if sc.Timeout <= 0 {
		return fallback
	}
	return time.Duration(sc.Timeout) * time.Millisecond
}
