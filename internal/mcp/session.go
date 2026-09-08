package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrManagerClosed indicates that the MCP manager has already been shut down.
var ErrManagerClosed = errors.New("MCP manager closed")

// ErrServerUnavailable indicates that a server could not be connected or
// reconnected. The original cause is wrapped for diagnostics.
var ErrServerUnavailable = errors.New("MCP server unavailable")

// ErrToolCallOutcomeUnknown indicates that an MCP tool call may have reached
// the remote server, but its response was lost. Such calls must not be
// replayed automatically unless the host explicitly opts into that policy.
var ErrToolCallOutcomeUnknown = errors.New("MCP tool call outcome unknown")

type serverSession struct {
	cfg ServerConfig

	sessionMu sync.RWMutex
	client    *sdkmcp.Client
	session   *sdkmcp.ClientSession
	lastInit  *sdkmcp.InitializeResult

	// reconnectMu serializes creation of a replacement session. Catalog
	// hydration intentionally happens outside this lock so a failed catalog
	// request can itself trigger recovery without deadlocking.
	reconnectMu sync.Mutex
	hydrateMu   sync.Mutex

	tools     []RemoteTool
	resources []RemoteResource
	templates []RemoteResourceTemplate
	prompts   []RemotePrompt

	subscriptionsMu      sync.Mutex
	subscriptions        map[string]*resourceSubscription
	desiredSubscriptions map[string]struct{}
}

type resourceSubscription struct {
	client  *sdkmcp.Client
	session *sdkmcp.ClientSession
}

func newServerSession(cfg ServerConfig) *serverSession {
	return &serverSession{
		cfg:                  cfg,
		subscriptions:        map[string]*resourceSubscription{},
		desiredSubscriptions: map[string]struct{}{},
	}
}

func (ss *serverSession) currentSession() *sdkmcp.ClientSession {
	if ss == nil {
		return nil
	}
	ss.sessionMu.RLock()
	defer ss.sessionMu.RUnlock()
	return ss.session
}

func (ss *serverSession) currentClient() *sdkmcp.Client {
	if ss == nil {
		return nil
	}
	ss.sessionMu.RLock()
	defer ss.sessionMu.RUnlock()
	return ss.client
}

func (ss *serverSession) ownsSession(session *sdkmcp.ClientSession) bool {
	return ss != nil && session != nil && ss.currentSession() == session
}

func (m *Manager) rootsSnapshot() []*sdkmcp.Root {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*sdkmcp.Root(nil), m.options.Roots...)
}

func (ss *serverSession) installSession(client *sdkmcp.Client, session *sdkmcp.ClientSession) {
	if ss == nil {
		return
	}
	ss.sessionMu.Lock()
	ss.client = client
	ss.session = session
	ss.lastInit = session.InitializeResult()
	ss.sessionMu.Unlock()
}

// invalidateSession clears only the expected session. A stale watcher must
// never be able to invalidate a newer replacement session.
func (ss *serverSession) invalidateSession(session *sdkmcp.ClientSession) bool {
	if ss == nil || session == nil {
		return false
	}
	ss.sessionMu.Lock()
	defer ss.sessionMu.Unlock()
	if ss.session != session {
		return false
	}
	ss.session = nil
	ss.client = nil
	return true
}

func (ss *serverSession) sessionInfo() *sdkmcp.InitializeResult {
	if ss == nil {
		return nil
	}
	ss.sessionMu.RLock()
	defer ss.sessionMu.RUnlock()
	if ss.session != nil && ss.session.InitializeResult() != nil {
		return ss.session.InitializeResult()
	}
	return ss.lastInit
}

func (ss *serverSession) desiredSubscriptionURIs() []string {
	if ss == nil {
		return nil
	}
	ss.subscriptionsMu.Lock()
	defer ss.subscriptionsMu.Unlock()
	uris := make([]string, 0, len(ss.desiredSubscriptions))
	for uri := range ss.desiredSubscriptions {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	return uris
}

func (ss *serverSession) markSubscriptionDesired(uri string) {
	ss.subscriptionsMu.Lock()
	if ss.desiredSubscriptions == nil {
		ss.desiredSubscriptions = map[string]struct{}{}
	}
	ss.desiredSubscriptions[uri] = struct{}{}
	ss.subscriptionsMu.Unlock()
}

func (ss *serverSession) unmarkSubscriptionDesired(uri string) {
	ss.subscriptionsMu.Lock()
	delete(ss.desiredSubscriptions, uri)
	ss.subscriptionsMu.Unlock()
}

func (ss *serverSession) subscriptionDesired(uri string) bool {
	ss.subscriptionsMu.Lock()
	defer ss.subscriptionsMu.Unlock()
	_, ok := ss.desiredSubscriptions[uri]
	return ok
}

func (ss *serverSession) takeSubscriptions() []*resourceSubscription {
	ss.subscriptionsMu.Lock()
	defer ss.subscriptionsMu.Unlock()
	subscriptions := make([]*resourceSubscription, 0, len(ss.subscriptions))
	for uri, subscription := range ss.subscriptions {
		delete(ss.subscriptions, uri)
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions
}

func (ss *serverSession) closeSubscriptions() error {
	if ss == nil {
		return nil
	}
	subscriptions := ss.takeSubscriptions()
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

func (ss *serverSession) removeSubscriptionIf(uri string, session *sdkmcp.ClientSession) bool {
	ss.subscriptionsMu.Lock()
	defer ss.subscriptionsMu.Unlock()
	current, ok := ss.subscriptions[uri]
	if !ok || current == nil || current.session != session {
		return false
	}
	delete(ss.subscriptions, uri)
	return true
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

func isSessionFailure(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, sdkmcp.ErrConnectionClosed) || errors.Is(err, sdkmcp.ErrSessionMissing) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"connection closed",
		"connection reset",
		"broken pipe",
		"server closing",
		"connection aborted",
		"session not found",
		"use of closed network connection",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func unavailableError(server, operation string, err error) error {
	if err == nil {
		err = ErrServerUnavailable
	}
	return fmt.Errorf("MCP %s on server %q unavailable: %w", operation, server, errors.Join(ErrServerUnavailable, err))
}

func unknownOutcomeError(tool string, err error) error {
	if err == nil {
		err = ErrToolCallOutcomeUnknown
	}
	return fmt.Errorf("MCP tool %q outcome unknown after connection failure: %w", tool, errors.Join(ErrToolCallOutcomeUnknown, err))
}

func (m *Manager) watchSession(ss *serverSession, session *sdkmcp.ClientSession) {
	if ss == nil || session == nil {
		return
	}
	err := session.Wait()
	if m == nil || m.closed.Load() {
		return
	}
	if ss.invalidateSession(session) {
		params := map[string]any{"server": ss.cfg.Name}
		if err != nil {
			params["err"] = err.Error()
		}
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server session lost", logging.LogOptions{Params: params})
	}
}

// ensureSession returns a live session, creating a replacement when the old
// one was lost. The session itself is ephemeral; the server configuration and
// desired host state are the durable inputs used to recreate it.
func (m *Manager) ensureSession(ctx context.Context, ss *serverSession) (*sdkmcp.ClientSession, error) {
	if m == nil || ss == nil {
		return nil, ErrServerUnavailable
	}
	if m.closed.Load() {
		return nil, ErrManagerClosed
	}
	if session := ss.currentSession(); session != nil {
		return session, nil
	}

	ss.reconnectMu.Lock()
	session := ss.currentSession()
	created := false
	var err error
	if session == nil {
		session, err = m.openAndInstallSession(ctx, ss, m.stderr)
		if err == nil {
			created = true
		}
	}
	ss.reconnectMu.Unlock()
	if err != nil {
		return nil, err
	}
	if created {
		go m.hydrateServer(context.WithoutCancel(ctx), ss, session)
	}
	current := ss.currentSession()
	if current == nil {
		return nil, ErrServerUnavailable
	}
	return current, nil
}

// hydrateServer reconstructs host-side state after a session replacement.
// Failures are logged and left to the normal per-operation recovery path; a
// connected server can still serve calls when only an optional catalog fails.
func (m *Manager) hydrateServer(ctx context.Context, ss *serverSession, session *sdkmcp.ClientSession) {
	if m == nil || m.closed.Load() || ss == nil || session == nil {
		return
	}
	ss.hydrateMu.Lock()
	defer ss.hydrateMu.Unlock()
	if ss.currentSession() != session {
		return
	}
	if err := m.registerServerCatalog(ctx, ss); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP server catalog hydration failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
	}
	if err := m.restoreSubscriptions(ctx, ss); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP subscription hydration failed", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": err.Error()}})
	}
}

func withSessionCall[T any](m *Manager, ctx context.Context, ss *serverSession, operation string, fn func(*sdkmcp.ClientSession) (T, error)) (T, error) {
	var zero T
	if ss == nil {
		return zero, ErrServerUnavailable
	}
	session, err := m.ensureSession(ctx, ss)
	if err != nil {
		return zero, unavailableError(ss.cfg.Name, operation, err)
	}
	result, err := fn(session)
	if err == nil {
		return result, nil
	}
	if !isSessionFailure(err) {
		return zero, err
	}
	m.invalidateAndCloseSession(ss, session, err)
	newSession, reconnectErr := m.ensureSession(ctx, ss)
	if reconnectErr != nil {
		return zero, unavailableError(ss.cfg.Name, operation, reconnectErr)
	}
	result, err = fn(newSession)
	if err != nil && isSessionFailure(err) {
		m.invalidateAndCloseSession(ss, newSession, err)
		return zero, unavailableError(ss.cfg.Name, operation, err)
	}
	return result, err
}

func (m *Manager) invalidateAndCloseSession(ss *serverSession, session *sdkmcp.ClientSession, cause error) {
	if ss == nil || session == nil || !ss.invalidateSession(session) {
		return
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "MCP session invalidated", logging.LogOptions{Params: map[string]any{"server": ss.cfg.Name, "err": cause.Error()}})
	_ = session.Close()
}

// toolCallMayRetry is intentionally conservative. MCP tool annotations are
// hints from an untrusted server, not a delivery guarantee. A missing session
// means the request was rejected before the tool could run, so replaying that
// specific case is safe. Hosts may opt into broader retries explicitly.
func (m *Manager) toolCallMayRetry(server, tool string, definition *sdkmcp.Tool, err error) bool {
	if errors.Is(err, sdkmcp.ErrSessionMissing) {
		return true
	}
	return m != nil && m.options.RetryToolCall != nil && m.options.RetryToolCall(server, tool, definition, err)
}
