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
